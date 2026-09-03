package modules

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSignedProfile(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	artifactPath := filepath.Join(t.TempDir(), "search.so")
	artifact := []byte("exact module artifact")
	if err := os.WriteFile(artifactPath, artifact, 0o600); err != nil {
		t.Fatal(err)
	}
	artifactSum := sha256.Sum256(artifact)
	profile := Profile{ProfileVersion: 1, Module: ModuleIdentity{Name: "search", Version: 1, RedisCompatibility: []string{"8.0"}, ArtifactSHA256: fmt.Sprintf("%x", artifactSum), ArtifactPath: artifactPath, VendorBuildID: "search-1"}, Commands: map[string]CommandRule{"FT.SEARCH": {ReadOnly: true, KeyModel: "index-name"}, "FT.CREATE": {ReadOnly: false, KeyModel: "index-name"}}, IssuedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}
	payload, _ := json.Marshal(profile)
	signature := ed25519.Sign(private, payload)
	envelope, _ := json.Marshal(Envelope{Payload: base64.StdEncoding.EncodeToString(payload), KeyID: "test", Signature: base64.StdEncoding.EncodeToString(signature)})
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, envelope, 0o600); err != nil {
		t.Fatal(err)
	}
	set, err := Load([]string{path}, map[string]string{"test": base64.StdEncoding.EncodeToString(public)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if set.Empty() || set.Digest() == "" {
		t.Fatal("profile set was not loaded")
	}
	if _, ok := set.Profile("SEARCH"); !ok {
		t.Fatal("profile lookup failed")
	}
	if err := os.WriteFile(artifactPath, []byte("changed module artifact"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load([]string{path}, map[string]string{"test": base64.StdEncoding.EncodeToString(public)}, now); err == nil {
		t.Fatal("expected module artifact hash rejection")
	}
	if err := os.WriteFile(artifactPath, artifact, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := set.ValidateAt(now.Add(2 * time.Hour)); err == nil {
		t.Fatal("expected profile expiry rejection")
	}
	envelope[len(envelope)-2] ^= 1
	_ = os.WriteFile(path, envelope, 0o600)
	if _, err := Load([]string{path}, map[string]string{"test": base64.StdEncoding.EncodeToString(public)}, now); err == nil {
		t.Fatal("expected tampered profile rejection")
	}
}

func TestValidateRejectsUnimplementedIndexPrefixAttestation(t *testing.T) {
	now := time.Now()
	profile := Profile{ProfileVersion: 1, Module: ModuleIdentity{Name: "search", Version: 1, RedisCompatibility: []string{"8.0"}, ArtifactSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ArtifactPath: "/opt/search.so", VendorBuildID: "build"}, Commands: map[string]CommandRule{"FT.SEARCH": {ReadOnly: true, KeyModel: "index-prefix-attested"}}, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute)}
	if err := validate(profile, now); err == nil {
		t.Fatal("expected unsupported index-prefix attestation rejection")
	}
}
