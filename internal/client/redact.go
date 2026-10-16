package client

import (
	"bytes"
	"encoding/json"
	"strings"
)

// RedactJSON masks password-like object keys. Non-JSON is returned unchanged.
func RedactJSON(body []byte) []byte {
	body = bytes.TrimSpace(body)
	if len(body) == 0 || !json.Valid(body) {
		return body
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return body
	}
	out, err := json.Marshal(redactAny(v))
	if err != nil {
		return body
	}
	return out
}

func redactAny(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if secretKey(k) {
				t[k] = "***"
				continue
			}
			t[k] = redactAny(val)
		}
		return t
	case []any:
		for i := range t {
			t[i] = redactAny(t[i])
		}
		return t
	default:
		return v
	}
}

func secretKey(k string) bool {
	switch strings.ToLower(k) {
	case "password", "adminpassword", "admin_password", "cookie", "set-cookie", "authorization", "token":
		return true
	default:
		return false
	}
}

// ResultCode reads a top-level numeric "code" field.
// PlumeLog's interceptor returns HTTP 200 with {"code":401}.
func ResultCode(body []byte) (int, bool) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 || body[0] != '{' {
		return 0, false
	}
	var probe struct {
		Code *float64 `json:"code"`
	}
	if err := json.Unmarshal(body, &probe); err != nil || probe.Code == nil {
		return 0, false
	}
	return int(*probe.Code), true
}
