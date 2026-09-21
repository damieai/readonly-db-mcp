package elasticsearch

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/your-org/readonly-db-mcp/internal/config"
)

type wire struct {
	pool    *http.Transport
	clients []elastictransport.Interface
	slots   chan struct{}
	cfg     *config.TargetConfig
	maxCell int
}
type leasedConn struct {
	net.Conn
	release         func()
	once            sync.Once
	mu              sync.Mutex
	active, expired bool
	timer           *time.Timer
}

func (c *leasedConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() {
		c.mu.Lock()
		if c.timer != nil {
			c.timer.Stop()
		}
		c.mu.Unlock()
		c.release()
	})
	return err
}
func (c *leasedConn) setActive(active bool) {
	c.mu.Lock()
	c.active = active
	closeNow := !active && c.expired
	c.mu.Unlock()
	if closeNow {
		_ = c.Close()
	}
}

func newWire(cfg *config.TargetConfig) (*wire, error) {
	secret, err := cfg.Password()
	if err != nil {
		return nil, failure("configuration_error", "cannot read Elasticsearch credentials")
	}
	ca, err := os.ReadFile(cfg.TLS.CAFile)
	if err != nil {
		return nil, failure("configuration_error", "cannot read Elasticsearch CA")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, failure("configuration_error", "invalid Elasticsearch CA")
	}
	baseTLS := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	if cfg.TLS.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			return nil, failure("configuration_error", "cannot load Elasticsearch client certificate")
		}
		baseTLS.Certificates = []tls.Certificate{cert}
	}
	w := &wire{slots: make(chan struct{}, cfg.Connection.MaxOpen), cfg: cfg}
	w.pool = &http.Transport{
		Proxy:              nil, // Deliberately ignore HTTP_PROXY/HTTPS_PROXY and node advertisements.
		DisableCompression: true, ForceAttemptHTTP2: false, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		MaxConnsPerHost: cfg.Connection.MaxOpen, MaxIdleConns: cfg.Connection.MaxIdle, MaxIdleConnsPerHost: cfg.Connection.MaxIdle,
		IdleConnTimeout: cfg.Connection.MaxIdleTime, ResponseHeaderTimeout: cfg.Connection.ReadTimeout,
		DisableKeepAlives: cfg.Connection.MaxIdle == 0, MaxResponseHeaderBytes: 64 << 10,
	}
	w.pool.DialTLSContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		select {
		case w.slots <- struct{}{}:
		default:
			// Free idle sockets on other configured nodes before waiting. A
			// per-host pool alone could deadlock at the target-wide ceiling.
			w.pool.CloseIdleConnections()
			select {
			case w.slots <- struct{}{}:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		release := func() { <-w.slots }
		dialCtx, cancel := context.WithTimeout(ctx, cfg.Connection.ConnectTimeout)
		defer cancel()
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			release()
			return nil, err
		}
		settings := baseTLS.Clone()
		settings.ServerName = host
		settings.NextProtos = []string{"http/1.1"}
		dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: cfg.Connection.ConnectTimeout, KeepAlive: 30 * time.Second}, Config: settings}
		conn, err := dialer.DialContext(dialCtx, network, address)
		if err != nil {
			release()
			return nil, err
		}
		leased := &leasedConn{Conn: conn, release: release, active: true}
		leased.mu.Lock()
		leased.timer = time.AfterFunc(cfg.Connection.MaxLifetime, func() {
			leased.mu.Lock()
			leased.expired = true
			closeNow := !leased.active
			leased.mu.Unlock()
			if closeNow {
				_ = leased.Close()
			}
		})
		leased.mu.Unlock()
		return leased, nil
	}
	// One SDK selector per configured origin, sharing a single HTTP/socket pool.
	// Startup verifies every origin; dispatch cannot expand this list or retry.
	for _, raw := range cfg.Elasticsearch.Endpoints {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			w.close()
			return nil, failure("configuration_error", "invalid Elasticsearch endpoint")
		}
		client, err := elastictransport.NewClient(elastictransport.WithURLs(u), elastictransport.WithBasicAuth(cfg.Username, secret), elastictransport.WithDisableRetry(), elastictransport.WithTransport(w.pool), elastictransport.WithUserAgent("readonly-db-mcp"))
		if err != nil {
			w.close()
			return nil, failure("configuration_error", "cannot initialize Elasticsearch transport")
		}
		w.clients = append(w.clients, client)
	}
	return w, nil
}

func (w *wire) get(ctx context.Context, endpoint int, path string, query url.Values, maxBytes int) ([]byte, error) {
	// No public method/path/host/header entry point. The wire stays package-private.
	u := &url.URL{Path: path, RawQuery: query.Encode()}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, failure("invalid_request", "cannot construct metadata request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "identity")
	var connection net.Conn
	trace := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			connection = info.Conn
			if conn, ok := connection.(*leasedConn); ok {
				conn.setActive(true)
			}
			_ = connection.SetWriteDeadline(time.Now().Add(w.cfg.Connection.WriteTimeout))
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			if connection != nil {
				_ = connection.SetWriteDeadline(time.Time{})
			}
		},
		PutIdleConn: func(error) {
			if conn, ok := connection.(*leasedConn); ok {
				conn.setActive(false)
			}
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(ctx, trace))
	response, err := w.clients[endpoint].Perform(req)
	if err != nil {
		if response != nil && response.Body != nil {
			response.Body.Close()
		}
		if ctx.Err() != nil {
			return nil, failure("deadline_exceeded", "Elasticsearch request was cancelled or timed out")
		}
		return nil, failure("transport_error", "Elasticsearch transport failed")
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return nil, failure("permission_denied", "Elasticsearch denied the configured identity")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, failure("upstream_error", "Elasticsearch metadata request failed")
	}
	if response.Header.Get("X-Elastic-Product") != "Elasticsearch" {
		return nil, failure("profile_mismatch", "upstream is not the pinned Elasticsearch product")
	}
	if encoding := response.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
		return nil, failure("invalid_response", "unexpected compressed Elasticsearch response")
	}
	if response.ContentLength > int64(maxBytes) {
		return nil, failure("resource_limit", "Elasticsearch response exceeds byte limit")
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, int64(maxBytes)+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, failure("deadline_exceeded", "Elasticsearch response was cancelled or timed out")
		}
		return nil, failure("transport_error", "cannot read Elasticsearch response")
	}
	if len(raw) > maxBytes {
		return nil, failure("resource_limit", "Elasticsearch response exceeds byte limit")
	}
	if err := strictJSONContext(ctx, raw, w.cfg.Elasticsearch.MaxJSONDepth, w.cfg.Elasticsearch.MaxJSONNodes, w.maxCell); err != nil {
		return nil, err
	}
	return raw, nil
}
func (w *wire) close() {
	if w.pool != nil {
		w.pool.CloseIdleConnections()
	}
}
