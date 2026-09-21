package sqlserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

const parserFrameLimit = 1 << 20

type parserReference struct {
	Parts []string `json:"parts"`
	Kind  string   `json:"kind"`
}
type parserResult struct {
	Protocol   int               `json:"protocol"`
	Accepted   bool              `json:"accepted"`
	References []parserReference `json:"references"`
	Parameters []string          `json:"parameters"`
	Nodes      int               `json:"nodes"`
	Error      string            `json:"error"`
}
type tsqlParser interface {
	Parse(context.Context, string, int, bool) (*parserResult, error)
}
type scriptDOM struct{ path string }

func parserPath(configured string) (string, error) {
	if configured == "" {
		binaryPath, err := os.Executable()
		if err != nil {
			return "", errors.New("locate SQL Server parser")
		}
		configured = filepath.Join(filepath.Dir(binaryPath), "sqlserver-parser", "readonly-sqlserver-parser")
	}
	info, err := os.Stat(configured)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", errors.New("SQL Server requires the built ScriptDom helper; configure sqlserver.parser_path")
	}
	return configured, nil
}

// boundedOutput stops allocation and aborts the exchange at the protocol limit.
type boundedOutput struct{ bytes.Buffer }

func (b *boundedOutput) Write(p []byte) (int, error) {
	if len(p) > parserFrameLimit+4-b.Len() {
		return 0, errors.New("parser response exceeds limit")
	}
	return b.Buffer.Write(p)
}

func (p scriptDOM) Parse(ctx context.Context, query string, compatibility int, module bool) (*parserResult, error) {
	request, err := json.Marshal(struct {
		SQL           string
		Compatibility int
		Module        bool
	}{query, compatibility, module})
	if err != nil || len(request) > parserFrameLimit {
		return nil, errors.New("SQL Server parser request exceeds limit")
	}
	var frame bytes.Buffer
	_ = binary.Write(&frame, binary.BigEndian, uint32(len(request)))
	frame.Write(request)
	cmd := exec.CommandContext(ctx, p.path)
	// No database credentials or inherited secret-bearing environment.
	cmd.Env = []string{"DOTNET_EnableDiagnostics=0", "DOTNET_CLI_TELEMETRY_OPTOUT=1", "COMPlus_GCHeapHardLimit=10000000"}
	cmd.Stdin = &frame
	var output boundedOutput
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("SQL Server ScriptDom helper failed")
	}
	data := output.Bytes()
	if len(data) < 4 || int(binary.BigEndian.Uint32(data[:4])) != len(data)-4 {
		return nil, errors.New("invalid SQL Server parser framing")
	}
	var result parserResult
	decoder := json.NewDecoder(bytes.NewReader(data[4:]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return nil, errors.New("invalid SQL Server parser response")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || result.Protocol != 1 || !result.Accepted || result.Nodes < 1 || result.Nodes > 100000 {
		return nil, errors.New("ScriptDom could not prove a read-only T-SQL operation")
	}
	return &result, nil
}

func (t *Target) parseQuery(ctx context.Context, query string, parameterCount int) (*core.Validation, error) {
	if t.parser == nil {
		return nil, errors.New("SQL Server ScriptDom parser is unavailable")
	}
	if len(query) > t.limits.MaxSQLBytes {
		return nil, errors.New("query exceeds the configured SQL size limit")
	}
	proof, err := t.parser.Parse(ctx, query, t.compatibility, false)
	if err != nil {
		return nil, err
	}
	params := map[int]struct{}{}
	for _, parameter := range proof.Parameters {
		position, ok := parameterPosition(strings.ToLower(parameter))
		if !ok {
			return nil, errors.New("SQL Server parameters must use @p1, @p2, ...")
		}
		params[position] = struct{}{}
	}
	if err := validateParameterSet(params, parameterCount); err != nil {
		return nil, err
	}
	validation := &core.Validation{Cacheable: false}
	sum := sha256.Sum256([]byte(query))
	validation.Fingerprint = hex.EncodeToString(sum[:12])
	for i, ref := range proof.References {
		// OBJECT_ID resolves a one-part name under the runtime user's actual
		// default schema, including SQL Server's dbo fallback and collation.
		if len(ref.Parts) == 1 {
			if ref.Kind != "relation" && ref.Kind != "function" || ref.Parts[0] == "" {
				return nil, errors.New("unresolved T-SQL identifier")
			}
			var schema, name string
			if err := t.db.QueryRowContext(ctx, `SELECT s.name,o.name FROM sys.objects o JOIN sys.schemas s ON s.schema_id=o.schema_id WHERE o.object_id=OBJECT_ID(@p1)`, "["+strings.ReplaceAll(ref.Parts[0], "]", "]]")+"]").Scan(&schema, &name); err != nil {
				return nil, errors.New("SQL Server entry object cannot be resolved")
			}
			ref.Parts = []string{schema, name}
			proof.References[i] = ref
		}
		schema, name, err := t.referenceName(ref)
		if err != nil {
			return nil, err
		}
		if err := t.policy.Load().allowSchema(schema); err != nil {
			return nil, err
		}
		if _, denied := t.denied[strings.ToLower(name)]; denied {
			return nil, errors.New("T-SQL references a denied entry object")
		}
		if _, denied := t.denied[strings.ToLower(schema+"."+name)]; denied {
			return nil, errors.New("T-SQL references a denied entry object")
		}
		validation.Tables = append(validation.Tables, schema+"."+name)
	}
	if t.verifyReferences == nil {
		return nil, errors.New("SQL Server module attestation is unavailable")
	}
	if err := t.verifyReferences(ctx, proof.References); err != nil {
		return nil, err
	}
	return validation, nil
}

func (t *Target) referenceName(ref parserReference) (string, string, error) {
	if ref.Kind != "relation" && ref.Kind != "function" {
		return "", "", errors.New("unknown T-SQL reference kind")
	}
	parts := ref.Parts
	for _, part := range parts {
		if part == "" {
			return "", "", errors.New("unresolved T-SQL identifier")
		}
	}
	if len(parts) == 3 && strings.EqualFold(parts[0], t.cfg.Database) {
		parts = parts[1:]
	}
	switch len(parts) {
	case 1:
		return t.defaultSchema, parts[0], nil
	case 2:
		return parts[0], parts[1], nil
	default:
		return "", "", errors.New("T-SQL reference escapes the configured database")
	}
}
