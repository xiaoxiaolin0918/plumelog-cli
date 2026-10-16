package client

import (
	"net/url"
	"strings"
)

// JoinURL builds base + prefix + path + query.
// base must not keep a trailing slash. prefix is "" or "/plumelog".
func JoinURL(base, prefix, path string, q url.Values) (string, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return "", errBase
	}
	if !strings.Contains(base, "://") {
		return "", errBase
	}
	path = "/" + strings.TrimLeft(path, "/")
	if prefix != "" {
		prefix = "/" + strings.Trim(prefix, "/")
		if prefix == "/" {
			prefix = ""
		}
	}
	u := base + prefix + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u, nil
}

// PathAlreadyPrefixed reports a path that already selected the /plumelog alias.
func PathAlreadyPrefixed(path string) bool {
	return path == "/plumelog" || strings.HasPrefix(path, "/plumelog/")
}

// RedactQuery returns a copy of q with adminPassWord masked.
func RedactQuery(q url.Values) url.Values {
	if q == nil {
		return nil
	}
	out := url.Values{}
	for k, vs := range q {
		if strings.EqualFold(k, "adminPassWord") {
			masked := make([]string, len(vs))
			for i := range vs {
				masked[i] = "***"
			}
			out[k] = masked
			continue
		}
		out[k] = append([]string(nil), vs...)
	}
	return out
}

// RedactURL masks adminPassWord in a fully built URL.
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if _, ok := q["adminPassWord"]; !ok {
		return raw
	}
	q.Set("adminPassWord", "***")
	u.RawQuery = q.Encode()
	return u.String()
}
