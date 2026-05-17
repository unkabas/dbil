package observ

// Helpers to coerce values returned by postgres.Pool.Execute (typed []any)
// into Go scalar types. pgx returns specific numeric types (int32 for int4,
// int64 for int8, float64 for float8, etc.); these helpers keep the
// collector code readable.

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

func asFloat64(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int64:
		return float64(t)
	case int32:
		return float64(t)
	case int:
		return float64(t)
	}
	return 0
}

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

func asIntArray(v any) []int {
	switch t := v.(type) {
	case []int32:
		out := make([]int, len(t))
		for i, x := range t {
			out[i] = int(x)
		}
		return out
	case []int64:
		out := make([]int, len(t))
		for i, x := range t {
			out[i] = int(x)
		}
		return out
	case []int:
		return append([]int(nil), t...)
	case []any:
		out := make([]int, 0, len(t))
		for _, x := range t {
			out = append(out, asInt(x))
		}
		return out
	}
	return nil
}
