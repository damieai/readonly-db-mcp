package admission

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/metrics"
)

type Class string

const (
	Metadata    Class = "metadata"
	Interactive Class = "interactive"
	Batch       Class = "batch"
	Maintenance Class = "maintenance"
)

var ErrOverloaded = errors.New("server busy")

type Config struct {
	Global, PerTarget, MaxQueued               int
	QueueTimeout                               time.Duration
	MetadataReserved, BatchMax, MaintenanceMax int
}

type waiter struct {
	target   string
	class    Class
	ready    chan struct{}
	canceled bool
}

type Controller struct {
	mu       sync.Mutex
	cfg      Config
	active   int
	byTarget map[string]int
	byClass  map[Class]int
	queues   map[Class][]*waiter
	next     int
	recorder metrics.Recorder
}

func (c *Controller) SetRecorder(r metrics.Recorder) { c.mu.Lock(); c.recorder = r; c.mu.Unlock() }

type Permit struct {
	c      *Controller
	target string
	class  Class
	once   sync.Once
}

func New(cfg Config) *Controller {
	return &Controller{cfg: cfg, byTarget: map[string]int{}, byClass: map[Class]int{}, queues: map[Class][]*waiter{Metadata: {}, Interactive: {}, Batch: {}, Maintenance: {}}}
}

func (c *Controller) Acquire(ctx context.Context, target string, class Class) (*Permit, error) {
	started := time.Now()
	c.mu.Lock()
	if c.totalQueuedLocked() >= c.cfg.MaxQueued {
		c.mu.Unlock()
		c.record(started, class, "rejected")
		return nil, ErrOverloaded
	}
	w := &waiter{target: target, class: class, ready: make(chan struct{})}
	c.queues[class] = append(c.queues[class], w)
	c.dispatchLocked()
	c.mu.Unlock()

	wait := c.cfg.QueueTimeout
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < wait {
		wait = time.Until(deadline)
	}
	if wait <= 0 {
		wait = time.Nanosecond
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-w.ready:
		c.record(started, class, "admitted")
		return &Permit{c: c, target: target, class: class}, nil
	case <-ctx.Done():
		c.cancel(w)
		c.record(started, class, "canceled")
		return nil, ctx.Err()
	case <-timer.C:
		c.cancel(w)
		c.record(started, class, "timeout")
		return nil, ErrOverloaded
	}
}

func (c *Controller) record(start time.Time, class Class, outcome string) {
	c.mu.Lock()
	r := c.recorder
	active := c.active
	queued := c.totalQueuedLocked()
	c.mu.Unlock()
	if r != nil {
		r.Observe("admission_wait", time.Since(start), string(class))
		r.Add("admission", 1, string(class), outcome)
		r.Set("admission_active", int64(active), string(class))
		r.Set("admission_queued", int64(queued), string(class))
	}
}

func (c *Controller) cancel(w *waiter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-w.ready:
		c.releaseLocked(w.target, w.class)
	default:
		w.canceled = true
		c.dispatchLocked()
	}
}

func (p *Permit) Release() {
	if p != nil {
		p.once.Do(func() { p.c.mu.Lock(); defer p.c.mu.Unlock(); p.c.releaseLocked(p.target, p.class) })
	}
}

func (c *Controller) releaseLocked(target string, class Class) {
	c.active--
	c.byTarget[target]--
	c.byClass[class]--
	c.dispatchLocked()
}

var order = []Class{Metadata, Interactive, Metadata, Interactive, Batch, Maintenance}

func (c *Controller) dispatchLocked() {
	for c.active < c.cfg.Global {
		granted := false
		for n := 0; n < len(order); n++ {
			idx := (c.next + n) % len(order)
			class := order[idx]
			q := c.queues[class]
			for len(q) > 0 && q[0].canceled {
				q = q[1:]
			}
			c.queues[class] = q
			candidate := -1
			for i, queued := range q {
				if !queued.canceled && c.canRunLocked(queued) {
					candidate = i
					break
				}
			}
			if candidate < 0 {
				continue
			}
			w := q[candidate]
			c.queues[class] = append(q[:candidate], q[candidate+1:]...)
			c.active++
			c.byTarget[w.target]++
			c.byClass[class]++
			c.next = (idx + 1) % len(order)
			close(w.ready)
			granted = true
			break
		}
		if !granted {
			return
		}
	}
}

func (c *Controller) canRunLocked(w *waiter) bool {
	if c.byTarget[w.target] >= c.cfg.PerTarget {
		return false
	}
	if w.class == Batch && c.byClass[Batch] >= c.cfg.BatchMax {
		return false
	}
	if w.class == Maintenance && c.byClass[Maintenance] >= c.cfg.MaintenanceMax {
		return false
	}
	if w.class != Metadata && c.cfg.MetadataReserved > 0 && c.hasRunnableLocked(Metadata) && c.active >= c.cfg.Global-c.cfg.MetadataReserved {
		return false
	}
	if w.class == Maintenance && c.hasRunnableLocked(Interactive) && c.active >= c.cfg.Global-1 {
		return false
	}
	return true
}

func (c *Controller) hasRunnableLocked(class Class) bool {
	for _, w := range c.queues[class] {
		if !w.canceled && c.byTarget[w.target] < c.cfg.PerTarget {
			return true
		}
	}
	return false
}
func (c *Controller) totalQueuedLocked() int {
	n := 0
	for _, q := range c.queues {
		for _, w := range q {
			if !w.canceled {
				n++
			}
		}
	}
	return n
}

type Stats struct{ Active, Queued int }

func (c *Controller) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{c.active, c.totalQueuedLocked()}
}
