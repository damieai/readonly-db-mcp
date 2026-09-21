// Package vectorproof implements RFC-0007 P1 catalog and native binding proof.
// Public query admission awaits runtime helper/deployment attestation and P2
// integration of execution and proof freshness on one lease.
package vectorproof

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const MaxObjects = 4096
const MaxCatalogBytes = 4 << 20
const SourceCommit = "cab9da72c04353f143bb06b42ab70a403daac64a"
const ExtensionVersion = "0.8.2"

//go:embed catalog.sql
var catalogSQL string

//go:embed profile-16-0.8.2.json
var profileJSON []byte

type Object struct {
	Class      string          `json:"class"`
	Identity   string          `json:"identity"`
	Definition json.RawMessage `json:"definition"`
	Member     bool            `json:"member"`
}

type Profile struct {
	Format          int      `json:"format"`
	PostgreSQLMajor int      `json:"postgresql_major"`
	Version         string   `json:"version"`
	SourceCommit    string   `json:"source_commit"`
	Objects         []Object `json:"objects"`
}

type Reference struct {
	Class string `json:"class"`
	OID   uint32 `json:"oid"`
}

type Snapshot struct {
	Schema       string
	ExtensionOID uint32
	databaseOID  uint32
	objects      map[Reference]Object
	digest       string
	tx           *sql.Tx
}

func (s *Snapshot) Digest() string { return s.digest }

// DenseTypeOIDs ties text codecs to the verified database-local type identities.
func (s *Snapshot) DenseTypeOIDs() [2]uint32 {
	var ids [2]uint32
	if s.digest == "" {
		return ids
	}
	for ref, obj := range s.objects {
		if ref.Class == "pg_type" {
			switch obj.Identity {
			case "@.vector":
				ids[0] = ref.OID
			case "@.halfvec":
				ids[1] = ref.OID
			}
		}
	}
	return ids
}

var schemaPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

func ReviewedProfile() (Profile, error) {
	var p Profile
	if err := json.Unmarshal(profileJSON, &p); err != nil {
		return p, err
	}
	if p.Format != 1 || p.PostgreSQLMajor != 16 || p.Version != ExtensionVersion || p.SourceCommit != SourceCommit || len(p.Objects) == 0 {
		return p, errors.New("invalid embedded pgvector profile")
	}
	return p, nil
}

// Capture only reads catalogs; it never generates authority from live metadata.
// The returned snapshot must be compared with a reviewed, source-pinned profile.
// Its transaction must have search_path=pg_catalog and a bounded deadline.
func Capture(ctx context.Context, tx *sql.Tx, schema string) (*Snapshot, error) {
	if !schemaPattern.MatchString(schema) || strings.HasPrefix(schema, "pg_") {
		return nil, errors.New("invalid pgvector installation schema")
	}
	var version, path string
	var major int
	var oid, databaseOID uint32
	err := tx.QueryRowContext(ctx, `SELECT e.oid,e.extversion,current_setting('server_version_num')::int/10000,current_setting('search_path'),(SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database()) FROM pg_catalog.pg_extension e JOIN pg_catalog.pg_namespace n ON n.oid=e.extnamespace WHERE e.extname='vector' AND n.nspname=$1`, schema).Scan(&oid, &version, &major, &path, &databaseOID)
	if err != nil || version != ExtensionVersion || major != 16 || path != "pg_catalog" {
		return nil, errors.New("pgvector proof requires PostgreSQL 16, vector 0.8.2 and catalog-only search path")
	}
	rows, err := tx.QueryContext(ctx, catalogSQL, schema)
	if err != nil {
		return nil, errors.New("read pgvector catalog")
	}
	defer rows.Close()
	snapshot := &Snapshot{Schema: schema, ExtensionOID: oid, databaseOID: databaseOID, objects: map[Reference]Object{}, tx: tx}
	bytes := 0
	for rows.Next() {
		var obj Object
		var ref Reference
		var definition sql.NullString
		if rows.Scan(&obj.Class, &ref.OID, &obj.Identity, &definition, &obj.Member) != nil {
			return nil, errors.New("invalid pgvector catalog row")
		}
		if !definition.Valid {
			return nil, errors.New("unreviewed pgvector object class")
		}
		ref.Class = obj.Class
		bytes += len(definition.String) + len(obj.Identity)
		if len(snapshot.objects) >= MaxObjects || bytes > MaxCatalogBytes {
			return nil, errors.New("pgvector catalog resource limit")
		}
		// Profiles use one fixed namespace placeholder. Only lower-case simple
		// namespace names are accepted; free-form query literals are never changed.
		obj.Identity = strings.ReplaceAll(obj.Identity, schema+".", "@.")
		var data any
		if json.Unmarshal([]byte(definition.String), &data) != nil {
			return nil, errors.New("invalid pgvector descriptor")
		}
		data = normalizeNamespace(data, schema)
		obj.Definition, _ = json.Marshal(data)
		if _, exists := snapshot.objects[ref]; exists {
			return nil, errors.New("duplicate pgvector catalog object")
		}
		snapshot.objects[ref] = obj
	}
	if rows.Err() != nil {
		return nil, errors.New("incomplete pgvector catalog")
	}
	if len(snapshot.objects) == 0 {
		return nil, errors.New("empty pgvector catalog")
	}
	return snapshot, nil
}

func normalizeNamespace(value any, schema string) any {
	switch v := value.(type) {
	case string:
		if v == schema {
			return "@"
		}
		return strings.ReplaceAll(v, schema+".", "@.")
	case map[string]any:
		for k, x := range v {
			v[k] = normalizeNamespace(x, schema)
		}
		return v
	case []any:
		for i, x := range v {
			v[i] = normalizeNamespace(x, schema)
		}
		return v
	default:
		return value
	}
}

func (s *Snapshot) Profile() Profile {
	p := Profile{Format: 1, PostgreSQLMajor: 16, Version: ExtensionVersion, SourceCommit: SourceCommit}
	for _, o := range s.objects {
		o.Definition = append(json.RawMessage(nil), o.Definition...)
		p.Objects = append(p.Objects, o)
	}
	sort.Slice(p.Objects, func(i, j int) bool { return objectKey(p.Objects[i]) < objectKey(p.Objects[j]) })
	return p
}

func objectKey(o Object) string { return o.Class + ":" + o.Identity }

func (s *Snapshot) Verify(p Profile) error {
	s.digest = ""
	if p.Format != 1 || p.PostgreSQLMajor != 16 || p.Version != ExtensionVersion || p.SourceCommit != SourceCommit || len(p.Objects) != len(s.objects) || len(p.Objects) > MaxObjects {
		return errors.New("pgvector profile identity or object inventory mismatch")
	}
	expected := map[string]Object{}
	for _, o := range p.Objects {
		key := objectKey(o)
		if _, exists := expected[key]; exists {
			return errors.New("duplicate profile object")
		}
		expected[key] = o
	}
	for _, o := range s.objects {
		key := objectKey(o)
		want, ok := expected[key]
		var a, b any
		if !ok || want.Member != o.Member || json.Unmarshal(want.Definition, &a) != nil || json.Unmarshal(o.Definition, &b) != nil {
			return errors.New("pgvector extension member mismatch")
		}
		aa, _ := json.Marshal(a)
		bb, _ := json.Marshal(b)
		if string(aa) != string(bb) {
			return fmt.Errorf("pgvector catalog definition mismatch (%s)", o.Class)
		}
		delete(expected, key)
	}
	if len(expected) != 0 {
		return errors.New("missing pgvector catalog objects")
	}
	encoded, _ := json.Marshal(s.Profile())
	hash := sha256.Sum256(encoded)
	s.digest = hex.EncodeToString(hash[:])
	return nil
}

// CheckExecuteGrants permits exact verified pgvector routine OIDs and any
// caller-attested helpers. It does not grant them, trust a schema, or accept
// functions merely marked IMMUTABLE.
func (s *Snapshot) CheckExecuteGrants(ctx context.Context, tx *sql.Tx, verifiedHelpers ...uint32) error {
	if s.digest == "" || tx == nil || s.tx != tx {
		return errors.New("pgvector snapshot has not been verified")
	}
	rows, err := tx.QueryContext(ctx, `SELECT p.oid::bigint FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND has_function_privilege(current_user,p.oid,'EXECUTE') LIMIT 4097`)
	if err != nil {
		return errors.New("inspect extension function privileges")
	}
	defer rows.Close()
	count := 0
	helpers := map[uint32]bool{}
	// Only caller-attested helper OIDs may be supplied here, never client input.
	for _, oid := range verifiedHelpers {
		helpers[oid] = true
	}
	for rows.Next() {
		var oid uint32
		count++
		if rows.Scan(&oid) != nil || count > MaxObjects {
			return errors.New("function privilege inventory limit")
		}
		if _, ok := s.objects[Reference{"pg_proc", oid}]; !ok && !helpers[oid] {
			return errors.New("role can execute an unreviewed non-system routine")
		}
	}
	if rows.Err() != nil {
		return errors.New("incomplete function privileges")
	}
	return nil
}
