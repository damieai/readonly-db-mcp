package neo4j

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	driver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	neo4jconfig "github.com/neo4j/neo4j-go-driver/v6/neo4j/config"
	"github.com/your-org/readonly-db-mcp/internal/admission"
	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
	"github.com/your-org/readonly-db-mcp/internal/metrics"
)

type Target struct {
	cfg        *config.TargetConfig
	limits     config.Limits
	query      driver.Driver
	attestor   driver.Driver
	controller *admission.Controller
	auditor    audit.Auditor
	recorder   metrics.Recorder
	mu         sync.Mutex
	checked    time.Time
	closed     bool
}

func Open(ctx context.Context, cfg *config.TargetConfig, limits config.Limits, controller *admission.Controller, auditor audit.Auditor, recorder metrics.Recorder) (*Target, error) {
	if cfg == nil || cfg.Neo4j == nil || cfg.Neo4j.Attestor == nil {
		return nil, errors.New("configuration_error: Neo4j target and attestor are required")
	}
	queryPassword, err := cfg.Password()
	if err != nil {
		return nil, errors.New("configuration_error: cannot read Neo4j query password")
	}
	attestorPassword, err := cfg.Neo4j.Attestor.Password()
	if err != nil {
		return nil, errors.New("configuration_error: cannot read Neo4j attestor password")
	}
	uriScheme := "bolt+s"
	if cfg.TLS.Mode == config.TLSDisabled {
		uriScheme = "bolt"
	}
	uri := uriScheme + "://" + net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	var roots *x509.CertPool
	if cfg.TLS.CAFile != "" {
		pem, err := os.ReadFile(cfg.TLS.CAFile)
		if err != nil {
			return nil, errors.New("configuration_error: cannot read Neo4j CA file")
		}
		roots = x509.NewCertPool()
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("configuration_error: invalid Neo4j CA file")
		}
	}
	query, err := newNeo4jDriver(uri, cfg, limits, roots, cfg.Username, queryPassword, cfg.Connection.MaxOpen)
	if err != nil {
		return nil, errors.New("configuration_error: cannot initialize Neo4j query driver")
	}
	attestor, err := newNeo4jDriver(uri, cfg, limits, roots, cfg.Neo4j.Attestor.Username, attestorPassword, 1)
	if err != nil {
		_ = query.Close(ctx)
		return nil, errors.New("configuration_error: cannot initialize Neo4j attestor driver")
	}
	t := &Target{cfg: cfg, limits: limits, query: query, attestor: attestor, controller: controller, auditor: auditor, recorder: recorder}
	proofCtx, cancel := context.WithTimeout(ctx, cfg.Connection.ConnectTimeout+30*time.Second)
	defer cancel()
	if err := t.prove(proofCtx); err != nil {
		_ = attestor.Close(proofCtx)
		_ = query.Close(proofCtx)
		return nil, fmt.Errorf("Neo4j target %q startup proof: %w", cfg.Name, err)
	}
	return t, nil
}

func newNeo4jDriver(uri string, cfg *config.TargetConfig, limits config.Limits, roots *x509.CertPool, username, password string, pool int) (driver.Driver, error) {
	return driver.NewDriver(uri, driver.BasicAuth(username, password, ""), func(c *neo4jconfig.Config) {
		c.MaxConnectionPoolSize = pool
		c.ConnectionAcquisitionTimeout = limits.MaxTimeout + time.Minute
		c.SocketConnectTimeout = cfg.Connection.ConnectTimeout
		c.MaxConnectionLifetime = cfg.Connection.MaxLifetime
		c.MaxTransactionRetryTime = 0
		c.DisableAutoCommitRetries = true
		c.FetchSize = min(limits.MaxRows+1, 256)
		c.TelemetryDisabled = true
		if cfg.TLS.Mode == config.TLSVerifyFull {
			c.TlsConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
		}
	})
}

func (t *Target) Info() core.TargetInfo {
	t.mu.Lock()
	ready := !t.closed && time.Since(t.checked) < t.cfg.Neo4j.PrivilegeRecheck
	checked := t.checked
	t.mu.Unlock()
	t.recordProofAge(checked)
	if t.recorder != nil {
		if ready {
			t.recorder.Set("neo4j_proof_ready", 1, t.cfg.Name)
		} else {
			t.recorder.Set("neo4j_proof_ready", 0, t.cfg.Name)
		}
	}
	return core.TargetInfo{Name: t.cfg.Name, Engine: config.EngineNeo4j, Environment: t.cfg.Environment, Consistency: t.cfg.Consistency, Database: t.cfg.Database, Healthy: ready, ReadOnlyUser: ready, ServerReadOnly: false, ParameterStyle: "$name", ServerVersion: t.cfg.Neo4j.Version, Capabilities: map[string]string{"cypher": "read,batch,explain,vector_search,graph_traversal", "metadata": "indexes,constraints,labels,relationship_types"}}
}

func (t *Target) Close() error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
	if t.recorder != nil {
		t.recorder.Set("neo4j_proof_ready", 0, t.cfg.Name)
		t.recorder.Set("neo4j_proof_age_ms", -1, t.cfg.Name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return errors.Join(t.query.Close(ctx), t.attestor.Close(ctx))
}
