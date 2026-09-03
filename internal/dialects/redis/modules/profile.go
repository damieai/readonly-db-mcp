package modules

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Envelope struct {
	Payload   string `json:"payload"`
	KeyID     string `json:"key_id"`
	Signature string `json:"signature"`
}

type Profile struct {
	ProfileVersion int                    `json:"profile_version"`
	Module         ModuleIdentity         `json:"module"`
	Commands       map[string]CommandRule `json:"commands"`
	IssuedAt       time.Time              `json:"issued_at"`
	ExpiresAt      time.Time              `json:"expires_at"`
}

type ModuleIdentity struct {
	Name               string   `json:"name"`
	Version            int      `json:"version"`
	RedisCompatibility []string `json:"redis_compatibility"`
	ArtifactSHA256     string   `json:"artifact_sha256"`
	ArtifactPath       string   `json:"artifact_path"`
	VendorBuildID      string   `json:"vendor_build_id"`
}

type CommandRule struct {
	ReadOnly            bool   `json:"readonly"`
	KeyModel            string `json:"key_model"`
	ExternalSideEffects bool   `json:"external_side_effects"`
}

type Set struct {
	profiles map[string]Profile
	digest   string
}

func Load(paths []string, trustedKeys map[string]string, now time.Time) (*Set, error) {
	set := &Set{profiles: map[string]Profile{}}
	if len(paths) == 0 {
		return set, nil
	}
	keys := make(map[string]ed25519.PublicKey, len(trustedKeys))
	for id, encoded := range trustedKeys {
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(key) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("trusted Redis module profile key %q is invalid", id)
		}
		keys[id] = ed25519.PublicKey(key)
	}
	var digestParts []string
	for _, path := range paths {
		data, err := readProfileFile(path)
		if err != nil {
			return nil, fmt.Errorf("read Redis module profile: %w", err)
		}
		var envelope Envelope
		dec := json.NewDecoder(strings.NewReader(string(data)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&envelope); err != nil {
			return nil, fmt.Errorf("decode Redis module profile: %w", err)
		}
		if err := requireJSONEOF(dec); err != nil {
			return nil, fmt.Errorf("decode Redis module profile: %w", err)
		}
		key, ok := keys[envelope.KeyID]
		if !ok {
			return nil, fmt.Errorf("Redis module profile uses untrusted key %q", envelope.KeyID)
		}
		payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
		if err != nil {
			return nil, errors.New("Redis module profile payload is invalid base64")
		}
		signature, err := base64.StdEncoding.DecodeString(envelope.Signature)
		if err != nil || !ed25519.Verify(key, payload, signature) {
			return nil, errors.New("Redis module profile signature is invalid")
		}
		var profile Profile
		payloadDecoder := json.NewDecoder(strings.NewReader(string(payload)))
		payloadDecoder.DisallowUnknownFields()
		if err := payloadDecoder.Decode(&profile); err != nil {
			return nil, fmt.Errorf("decode signed Redis module profile payload: %w", err)
		}
		if err := requireJSONEOF(payloadDecoder); err != nil {
			return nil, fmt.Errorf("decode signed Redis module profile payload: %w", err)
		}
		if err := validate(profile, now); err != nil {
			return nil, err
		}
		if err := verifyArtifact(profile.Module.ArtifactPath, profile.Module.ArtifactSHA256); err != nil {
			return nil, fmt.Errorf("verify Redis module profile artifact: %w", err)
		}
		name := strings.ToLower(profile.Module.Name)
		if _, duplicate := set.profiles[name]; duplicate {
			return nil, fmt.Errorf("duplicate Redis module profile %q", name)
		}
		set.profiles[name] = profile
		digestParts = append(digestParts, envelope.Payload+"\x00"+envelope.KeyID)
	}
	set.digest = digestStrings(digestParts)
	return set, nil
}

func verifyArtifact(path, expected string) error {
	linkInfo, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 || !linkInfo.Mode().IsRegular() || linkInfo.Size() > 256<<20 {
		return errors.New("module artifact must be a regular non-symlink file no larger than 256 MiB")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !os.SameFile(linkInfo, openedInfo) || !openedInfo.Mode().IsRegular() || openedInfo.Size() > 256<<20 {
		return errors.New("module artifact changed while it was being opened")
	}
	h := sha256.New()
	written, err := io.Copy(h, io.LimitReader(file, (256<<20)+1))
	if err != nil {
		return err
	}
	if written > 256<<20 {
		return errors.New("module artifact exceeds 256 MiB")
	}
	if !strings.EqualFold(fmt.Sprintf("%x", h.Sum(nil)), expected) {
		return errors.New("module artifact SHA-256 does not match the signed profile")
	}
	return nil
}

func readProfileFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	const maximum = 1 << 20
	if !info.Mode().IsRegular() || info.Size() > maximum {
		return nil, errors.New("module profile must be a regular file no larger than 1 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maximum {
		return nil, errors.New("module profile exceeds 1 MiB")
	}
	return data, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func validate(profile Profile, now time.Time) error {
	if profile.ProfileVersion != 1 {
		return errors.New("unsupported Redis module profile version")
	}
	if profile.Module.Name == "" || profile.Module.Version < 1 || !regexp.MustCompile(`^[a-fA-F0-9]{64}$`).MatchString(profile.Module.ArtifactSHA256) || !filepath.IsAbs(profile.Module.ArtifactPath) || profile.Module.VendorBuildID == "" || len(profile.Module.RedisCompatibility) == 0 {
		return errors.New("Redis module profile identity is incomplete")
	}
	if profile.IssuedAt.IsZero() || profile.ExpiresAt.IsZero() || now.Before(profile.IssuedAt) || !now.Before(profile.ExpiresAt) {
		return errors.New("Redis module profile is not currently valid")
	}
	if len(profile.Commands) == 0 {
		return errors.New("Redis module profile has no commands")
	}
	for name, rule := range profile.Commands {
		if name == "" || strings.ToUpper(name) != name {
			return errors.New("Redis module profile command names must be uppercase")
		}
		switch rule.KeyModel {
		case "ordinary-keys", "index-name", "keyless-safe":
		case "index-prefix-attested":
			return fmt.Errorf("Redis module command %q requires a module-specific index-prefix verifier that is not implemented", name)
		default:
			return fmt.Errorf("Redis module command %q has unknown key model", name)
		}
		if rule.ReadOnly && rule.ExternalSideEffects {
			return fmt.Errorf("Redis module command %q cannot be admitted as read-only with external side effects", name)
		}
	}
	return nil
}

func (s *Set) Empty() bool { return s == nil || len(s.profiles) == 0 }
func (s *Set) Digest() string {
	if s == nil {
		return ""
	}
	return s.digest
}
func (s *Set) Profile(name string) (Profile, bool) {
	if s == nil {
		return Profile{}, false
	}
	p, ok := s.profiles[strings.ToLower(name)]
	return p, ok
}

func (s *Set) Profiles() []Profile {
	if s == nil {
		return nil
	}
	result := make([]Profile, 0, len(s.profiles))
	for _, profile := range s.profiles {
		result = append(result, profile)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Module.Name) < strings.ToLower(result[j].Module.Name)
	})
	return result
}

func (s *Set) ValidateAt(now time.Time) error {
	if s == nil {
		return nil
	}
	for _, profile := range s.profiles {
		if err := validate(profile, now); err != nil {
			return err
		}
	}
	return nil
}

func digestStrings(values []string) string {
	sort.Strings(values)
	h := sha256.New()
	for _, value := range values {
		_, _ = h.Write([]byte(value))
		_, _ = h.Write([]byte{0})
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
