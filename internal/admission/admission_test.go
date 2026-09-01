package admission

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCapacityAndRelease(t *testing.T) {
	c := New(Config{Global: 1, PerTarget: 1, MaxQueued: 2, QueueTimeout: 20 * time.Millisecond, BatchMax: 1, MaintenanceMax: 1})
	p, err := c.Acquire(context.Background(), "a", Interactive)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Acquire(context.Background(), "a", Interactive)
	if !errors.Is(err, ErrOverloaded) {
		t.Fatalf("got %v", err)
	}
	p.Release()
	p.Release()
	if s := c.Stats(); s.Active != 0 || s.Queued != 0 {
		t.Fatalf("leaked: %#v", s)
	}
}

func TestCanceledWaitDoesNotLeak(t *testing.T) {
	c := New(Config{Global: 1, PerTarget: 1, MaxQueued: 2, QueueTimeout: time.Second, BatchMax: 1, MaintenanceMax: 1})
	p, _ := c.Acquire(context.Background(), "a", Interactive)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Acquire(ctx, "b", Interactive); err == nil {
		t.Fatal("expected cancellation")
	}
	p.Release()
	if s := c.Stats(); s.Active != 0 || s.Queued != 0 {
		t.Fatalf("leaked: %#v", s)
	}
}

func TestBlockedTargetDoesNotHeadOfLineBlockAnotherTarget(t *testing.T) {
	c := New(Config{Global: 2, PerTarget: 1, MaxQueued: 4, QueueTimeout: time.Second, BatchMax: 2, MaintenanceMax: 1})
	p, _ := c.Acquire(context.Background(), "busy", Interactive)
	done := make(chan *Permit, 1)
	go func() { permit, _ := c.Acquire(context.Background(), "busy", Interactive); done <- permit }()
	time.Sleep(5 * time.Millisecond)
	other, err := c.Acquire(context.Background(), "other", Interactive)
	if err != nil {
		t.Fatal(err)
	}
	other.Release()
	p.Release()
	(<-done).Release()
}

func TestCanceledWaiterBehindBlockedHeadIsNeverGranted(t *testing.T) {
	c := New(Config{Global: 1, PerTarget: 1, MaxQueued: 4, QueueTimeout: time.Second, BatchMax: 1, MaintenanceMax: 1})
	active, _ := c.Acquire(context.Background(), "busy", Interactive)
	next := make(chan *Permit, 1)
	go func() { p, _ := c.Acquire(context.Background(), "busy", Interactive); next <- p }()
	time.Sleep(5 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Acquire(ctx, "other", Interactive); err == nil {
		t.Fatal("expected cancellation")
	}
	active.Release()
	(<-next).Release()
	if s := c.Stats(); s.Active != 0 || s.Queued != 0 {
		t.Fatalf("leaked: %#v", s)
	}
}
