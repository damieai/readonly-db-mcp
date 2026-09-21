package redis

import (
	"errors"
	"strconv"
	"strings"

	"github.com/your-org/readonly-db-mcp/internal/core"
)

// Summaries use the returned FT.INFO document, after checking its scope again.
// They do not constrain query syntax, algorithms or vector dtypes to a whitelist.
func (p *Policy) searchSummary(command string, raw any, maxCell int) (*core.RedisSearchIndex, error) {
	if !strings.EqualFold(command, "FT.INFO") {
		return nil, nil
	}
	name, err := p.checkSearchIndex(raw, command)
	if err != nil {
		return nil, err
	}
	if len(name) > maxCell {
		return nil, errors.New("Redis Search metadata exceeds the cell limit")
	}
	info, _ := strictModuleMap(raw)
	definition, _ := strictModuleMap(info["index_definition"])
	storage, _ := moduleString(definition["key_type"])
	attributes, ok := info["attributes"].([]any)
	if !ok || len(attributes) > p.cfg.MaxKeysPerCommand {
		return nil, errors.New("Redis Search attributes are missing or exceed the metadata limit")
	}
	result := &core.RedisSearchIndex{Name: name, Storage: storage, Vectors: []core.RedisVectorField{}}
	for _, rawAttribute := range attributes {
		// Ordinary fields can have standalone flag tokens (SORTABLE, UNF, ...).
		// Vector field metadata consists of pairs; identify type before parsing.
		isVector := false
		if values, ok := rawAttribute.([]any); ok {
			for i := 0; i+1 < len(values); i++ {
				key, _ := moduleString(values[i])
				value, _ := moduleString(values[i+1])
				if key == "type" && value == "VECTOR" {
					isVector = true
				}
			}
		} else if fields, err := strictModuleMap(rawAttribute); err == nil {
			kind, _ := moduleString(fields["type"])
			isVector = kind == "VECTOR"
		}
		if !isVector {
			continue
		}
		fields, err := strictModuleMap(rawAttribute)
		if err != nil {
			return nil, err
		}
		field := core.RedisVectorField{}
		for key, dst := range map[string]*string{"identifier": &field.Identifier, "attribute": &field.Attribute, "algorithm": &field.Algorithm, "data_type": &field.DataType, "distance_metric": &field.DistanceMetric} {
			value, ok := moduleString(fields[key])
			if !ok || value == "" || len(value) > min(maxCell, p.cfg.MaxArgumentBytes) {
				return nil, errors.New("Redis vector metadata has an invalid field")
			}
			*dst = value
		}
		var dim int64
		switch value := fields["dim"].(type) {
		case int64:
			dim = value
		case string:
			dim, err = strconv.ParseInt(value, 10, 32)
		default:
			return nil, errors.New("Redis vector dimensions have an invalid shape")
		}
		if err != nil || dim < 1 || dim > 1<<31-1 {
			return nil, errors.New("Redis vector dimensions are invalid")
		}
		field.Dimensions = int(dim)
		result.Vectors = append(result.Vectors, field)
	}
	return result, nil
}
