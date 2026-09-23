package elasticsearch

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/core"
)

func validateScriptMetadataRequest(request core.ElasticsearchMetadataRequest, cfg *config.ElasticsearchConfig) error {
	if len(request.Indices) != 0 || len(request.Fields) != 0 || len(request.Names) != 1 {
		return failure("invalid_request", "get_script requires one script ID and no index or field selectors")
	}
	for _, allowed := range cfg.ReadableScriptIDs {
		if request.Names[0] == allowed {
			return nil
		}
	}
	return failure("scope_denied", "stored script ID is outside the operator allowlist")
}

func (t *Target) readScriptMetadata(ctx context.Context, endpoint int, id string, options url.Values) ([]byte, error) {
	if err := t.ready(); err != nil {
		return nil, err
	}
	left := time.Until(deadline(ctx)).Milliseconds()
	if left < 1 {
		return nil, failure("deadline_exceeded", "metadata deadline expired")
	}
	if supplied := options.Get("master_timeout"); supplied != "" && supplied != "-1" {
		duration, err := time.ParseDuration(supplied)
		if err != nil || duration <= 0 {
			return nil, failure("invalid_request", "get_script master_timeout requires a positive duration or -1")
		}
		if requested := max(int64(1), duration.Milliseconds()); requested < left {
			left = requested
		}
	}
	options.Set("master_timeout", strconv.FormatInt(left, 10)+"ms")
	raw, err := t.wire.request(ctx, endpoint, http.MethodGet, "/_scripts/"+url.PathEscape(id), options, nil, "application/json", t.limits.MaxResultBytes, false)
	if err != nil {
		return nil, err
	}
	if err := strictJSONContext(ctx, raw, t.cfg.Elasticsearch.MaxJSONDepth, t.cfg.Elasticsearch.MaxJSONNodes, 0); err != nil {
		return nil, err
	}
	response, err := decodeObject(raw)
	if err != nil {
		return nil, failure("invalid_response", "stored script response is not an object")
	}
	if response["_id"] != id || response["found"] != true {
		return nil, failure("invalid_response", "stored script response does not identify the requested script")
	}
	script, err := object(response["script"])
	if err != nil {
		return nil, failure("invalid_response", "stored script definition is missing")
	}
	if lang, ok := script["lang"].(string); !ok || lang == "" {
		return nil, failure("invalid_response", "stored script language is missing")
	}
	if _, ok := script["source"].(string); !ok {
		if _, err := object(script["source"]); err != nil {
			return nil, failure("invalid_response", "stored script source is missing")
		}
	}
	return raw, nil
}
