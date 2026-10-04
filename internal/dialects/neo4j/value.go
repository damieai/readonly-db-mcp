package neo4j

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"

	"github.com/neo4j/neo4j-go-driver/v6/neo4j/dbtype"
)

const maxSafeInteger = 9007199254740991

func normalize(value any, maxCell, depth int) (any, error) {
	if depth > 32 {
		return nil, errors.New("resource_limit: Neo4j value nesting exceeds limit")
	}
	switch v := value.(type) {
	case nil, bool, int, int8, int16, int32, uint8, uint16, uint32:
		return v, nil
	case int64:
		if v > maxSafeInteger || v < -maxSafeInteger {
			return strconv.FormatInt(v, 10), nil
		}
		return v, nil
	case uint64:
		if v > maxSafeInteger {
			return strconv.FormatUint(v, 10), nil
		}
		return v, nil
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return normalize(i, maxCell, depth)
		}
		f, err := v.Float64()
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, errors.New("invalid_request: Neo4j numeric value is invalid")
		}
		return f, nil
	case float32:
		return normalize(float64(v), maxCell, depth)
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.New("invalid_response: Neo4j non-finite float")
		}
		return v, nil
	case string:
		if len(v) > maxCell {
			return nil, errors.New("resource_limit: Neo4j cell exceeds max_cell_bytes")
		}
		return v, nil
	case []byte:
		if len(v) > maxCell {
			return nil, errors.New("resource_limit: Neo4j cell exceeds max_cell_bytes")
		}
		return map[string]any{"base64": base64.StdEncoding.EncodeToString(v)}, nil
	case time.Time:
		return v.Format(time.RFC3339Nano), nil
	case dbtype.Date, dbtype.Time, dbtype.LocalTime, dbtype.LocalDateTime, dbtype.Duration, dbtype.UUID:
		return fmt.Sprint(v), nil
	case dbtype.Point2D:
		return map[string]any{"type": "point2d", "srid": v.SpatialRefId, "x": v.X, "y": v.Y}, nil
	case dbtype.Point3D:
		return map[string]any{"type": "point3d", "srid": v.SpatialRefId, "x": v.X, "y": v.Y, "z": v.Z}, nil
	case dbtype.Node:
		props, err := normalize(v.Props, maxCell, depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "node", "element_id": v.ElementId, "labels": v.Labels, "properties": props}, nil
	case dbtype.Relationship:
		props, err := normalize(v.Props, maxCell, depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "relationship", "element_id": v.ElementId, "start_element_id": v.StartElementId, "end_element_id": v.EndElementId, "relationship_type": v.Type, "properties": props}, nil
	case dbtype.Path:
		nodes, err := normalize(v.Nodes, maxCell, depth+1)
		if err != nil {
			return nil, err
		}
		rels, err := normalize(v.Relationships, maxCell, depth+1)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "path", "nodes": nodes, "relationships": rels}, nil
	case dbtype.Vector[float64]:
		return vectorValue("float64", v.Elems, maxCell, depth)
	case dbtype.Vector[float32]:
		return vectorValue("float32", v.Elems, maxCell, depth)
	case dbtype.Vector[int8]:
		return vectorValue("int8", v.Elems, maxCell, depth)
	case dbtype.Vector[int16]:
		return vectorValue("int16", v.Elems, maxCell, depth)
	case dbtype.Vector[int32]:
		return vectorValue("int32", v.Elems, maxCell, depth)
	case dbtype.Vector[int64]:
		return vectorValue("int64", v.Elems, maxCell, depth)
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		if rv.Len() > 10000 {
			return nil, errors.New("resource_limit: Neo4j list is too long")
		}
		out := make([]any, rv.Len())
		for i := range out {
			item, err := normalize(rv.Index(i).Interface(), maxCell, depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = item
		}
		return out, nil
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String || rv.Len() > 10000 {
			return nil, errors.New("invalid_response: Neo4j map shape is unsupported")
		}
		out := make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			key := iter.Key().String()
			if len(key) > maxCell {
				return nil, errors.New("resource_limit: Neo4j map key is too long")
			}
			item, err := normalize(iter.Value().Interface(), maxCell, depth+1)
			if err != nil {
				return nil, err
			}
			out[key] = item
		}
		return out, nil
	}
	return nil, fmt.Errorf("invalid_response: unsupported Neo4j value type %T", value)
}

func vectorValue[T dbtype.VectorElement](kind string, values []T, maxCell, depth int) (any, error) {
	if len(values) > 10000 {
		return nil, errors.New("resource_limit: Neo4j vector is too long")
	}
	out, err := normalize(values, maxCell, depth+1)
	if err != nil {
		return nil, err
	}
	return map[string]any{"type": "vector", "element_type": kind, "values": out}, nil
}
