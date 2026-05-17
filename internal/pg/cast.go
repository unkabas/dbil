package pg

// Helpers to coerce pgx scalar values (returned through postgres.Pool.Execute
// as typed []any) into Go scalars. Same logic as internal/observ/cast.go but
// duplicated here so the two packages stay independent.

func asInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int32:
		return int64(t)
	case int16:
		return int64(t)
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case float32:
		return int64(t)
	}
	return 0
}

func asInt(v any) int { return int(asInt64(v)) }

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return ""
}

func asBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
