package neo4j

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

// A local TLS peer verifies the driver's actual bolt+s handshake. It closes
// after TLS because no Neo4j protocol server is available in this unit test.
func TestNeo4jDriverVerifiedTLS(t *testing.T) {
	certificate, roots := testNeo4jCertificate(t)
	for _, tc := range []struct {
		name, host string
		roots      *x509.CertPool
		accept     bool
	}{
		{name: "trusted_ca_and_hostname", host: "localhost", roots: roots, accept: true},
		{name: "untrusted_ca", host: "localhost"},
		{name: "wrong_hostname", host: "127.0.0.1", roots: roots},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
				Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			handshake := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					handshake <- err
					return
				}
				defer conn.Close()
				handshake <- conn.(*tls.Conn).Handshake()
			}()
			port := listener.Addr().(*net.TCPAddr).Port
			uri := "bolt+s://" + net.JoinHostPort(tc.host, strconv.Itoa(port))
			cfg := &config.TargetConfig{TLS: config.TLSConfig{Mode: config.TLSVerifyFull}}
			cfg.Connection.ConnectTimeout = time.Second
			connection, err := newNeo4jDriver(uri, cfg, config.Limits{MaxTimeout: time.Second, MaxRows: 1}, tc.roots, "query", "secret", 1)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			defer connection.Close(ctx)
			if err := connection.VerifyConnectivity(ctx); err == nil {
				t.Fatal("fake Bolt server unexpectedly completed connectivity")
			}
			select {
			case err := <-handshake:
				if (err == nil) != tc.accept {
					t.Fatalf("TLS handshake accepted=%t, want %t: %v", err == nil, tc.accept, err)
				}
			case <-time.After(time.Second):
				t.Fatal("driver did not attempt a TLS handshake")
			}
		})
	}
}

func testNeo4jCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test Neo4j CA"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "localhost"},
		DNSNames: []string{"localhost"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, ca, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})) {
		t.Fatal("cannot add test CA")
	}
	return tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: leafKey}, roots
}
