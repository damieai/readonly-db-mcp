package registry

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	mysqltarget "github.com/your-org/readonly-db-mcp/internal/dialects/mysql"
	postgresqltarget "github.com/your-org/readonly-db-mcp/internal/dialects/postgresql"
	redistarget "github.com/your-org/readonly-db-mcp/internal/dialects/redis"
	sqlservertarget "github.com/your-org/readonly-db-mcp/internal/dialects/sqlserver"
	"github.com/your-org/readonly-db-mcp/internal/metrics"
)

type Registry struct {
	mu      sync.RWMutex
	targets map[string]core.Target
}

func Open(ctx context.Context, cfg *config.Config, auditor audit.Auditor, recorder metrics.Recorder) (*Registry, error) {
	registry := &Registry{targets: make(map[string]core.Target, len(cfg.Targets))}
	controller := admission.New(admission.Config{
		Global: cfg.Limits.GlobalConcurrency, PerTarget: cfg.Limits.PerTargetConcurrency,
		MaxQueued: cfg.Limits.MaxQueuedRequests, QueueTimeout: cfg.Limits.QueueTimeout,
		MetadataReserved: cfg.Limits.WorkloadClasses.MetadataReserved,
		BatchMax:         cfg.Limits.WorkloadClasses.BatchMaxConcurrency,
		MaintenanceMax:   cfg.Limits.WorkloadClasses.MaintenanceMaxConcurrency,
	})
	controller.SetRecorder(recorder)
	names := make([]string, 0, len(cfg.Targets))
	for name := range cfg.Targets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		targetCfg := cfg.Targets[name]
		var target core.Target
		var err error
		switch targetCfg.Engine {
		case config.EngineMySQL:
			target, err = mysqltarget.Open(ctx, targetCfg, cfg.Limits, controller, auditor, recorder)
		case config.EnginePostgreSQL:
			target, err = postgresqltarget.Open(ctx, targetCfg, cfg.Limits, controller, auditor, recorder)
		case config.EngineSQLServer:
			target, err = sqlservertarget.Open(ctx, targetCfg, cfg.Limits, controller, auditor, recorder)
		case config.EngineRedis:
			target, err = redistarget.Open(ctx, targetCfg, cfg.Limits, controller, auditor, recorder)
		default:
			err = fmt.Errorf("unsupported database engine %q", targetCfg.Engine)
		}
		if err != nil {
			_ = registry.Close()
			return nil, err
		}
		registry.targets[name] = target
	}
	return registry, nil
}

func (r *Registry) GetSQL(name string) (core.SQLTarget, error) {
	target, err := r.Get(name)
	if err != nil {
		return nil, err
	}
	sqlTarget, ok := target.(core.SQLTarget)
	if !ok {
		return nil, errors.New("selected target is not a SQL database")
	}
	return sqlTarget, nil
}

func (r *Registry) GetRedis(name string) (core.RedisTarget, error) {
	target, err := r.Get(name)
	if err != nil {
		return nil, err
	}
	redisTarget, ok := target.(core.RedisTarget)
	if !ok {
		return nil, errors.New("selected target is not Redis")
	}
	return redisTarget, nil
}

func (r *Registry) Get(name string) (core.Target, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if name == "" {
		return nil, errors.New("target is required")
	}
	target, ok := r.targets[name]
	if !ok {
		return nil, fmt.Errorf("target %q is not configured; call list_targets", name)
	}
	return target, nil
}

func (r *Registry) List() []core.TargetInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]core.TargetInfo, 0, len(r.targets))
	for _, target := range r.targets {
		result = append(result, target.Info())
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var errs []error
	for name, target := range r.targets {
		if err := target.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close target %q: %w", name, err))
		}
		delete(r.targets, name)
	}
	return errors.Join(errs...)
}
