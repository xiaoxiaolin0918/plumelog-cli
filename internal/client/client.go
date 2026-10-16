package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"plumelog-cli/internal/output"
	"plumelog-cli/internal/version"
)

// errBase is returned when the server root is unusable.
var errBase = errors.New("base-url must be an absolute URL without a trailing slash, for example http://localhost:8891")

// Request is one PlumeLog HTTP call before prefix resolution.
type Request struct {
	Method  string
	Path    string
	Query   url.Values
	Body    []byte
	Headers map[string]string
}

// Response is the upstream reply from the prefix that was actually used.
type Response struct {
	Status   int
	Body     []byte
	FinalURL string
	Header   http.Header
}

// Client talks to one PlumeLog server and keeps the session cookie jar.
type Client struct {
	BaseURL  string
	Prefixes []string
	HTTP     *http.Client
	Jar      *Jar
}

// Preview describes a dry-run. Secrets in the body and adminPassWord are masked.
func (c *Client) Preview(req Request, cookieNames []string, wouldRequire []string, risk string) (output.Envelope, error) {
	urls, err := c.PreviewURLs(req)
	if err != nil {
		return output.Envelope{}, err
	}
	env := output.Envelope{
		OK:           true,
		DryRun:       true,
		Risk:         risk,
		Method:       strings.ToUpper(req.Method),
		URL:          urls[0],
		Headers:      previewHeaders(req, cookieNames),
		CookieNames:  cookieNames,
		WouldRequire: wouldRequire,
	}
	if len(urls) > 1 {
		env.FallbackURLs = urls[1:]
	}
	if len(bytes.TrimSpace(req.Body)) > 0 {
		redacted := RedactJSON(req.Body)
		if jsonBody(redacted) {
			env.Body = redacted
		} else {
			env.BodyText = string(redacted)
		}
	}
	return env, nil
}

func previewHeaders(req Request, cookieNames []string) map[string]string {
	h := map[string]string{
		"Accept":     "application/json, text/plain, */*",
		"User-Agent": version.UserAgent,
	}
	if len(bytes.TrimSpace(req.Body)) > 0 {
		h["Content-Type"] = "application/json"
	}
	for k, v := range req.Headers {
		if strings.EqualFold(k, "cookie") || strings.EqualFold(k, "authorization") {
			h[k] = "<redacted>"
			continue
		}
		h[k] = v
	}
	if len(cookieNames) > 0 {
		h["Cookie"] = "<redacted>"
	}
	return h
}

func jsonBody(b []byte) bool {
	b = bytes.TrimSpace(b)
	return len(b) > 0 && (b[0] == '{' || b[0] == '[')
}

// Do sends the request, trying each path prefix until one is not HTTP 404.
func (c *Client) Do(ctx context.Context, req Request) (Response, error) {
	urls, err := c.candidateURLs(req)
	if err != nil {
		return Response{}, err
	}
	var last Response
	for i, raw := range urls {
		resp, err := c.doOne(ctx, req, raw)
		if err != nil {
			return Response{}, err
		}
		last = resp
		if resp.Status == http.StatusNotFound && i < len(urls)-1 {
			continue
		}
		return resp, nil
	}
	return last, nil
}

func (c *Client) doOne(ctx context.Context, req Request, rawURL string) (Response, error) {
	method := strings.ToUpper(req.Method)
	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Accept", "application/json, text/plain, */*")
	httpReq.Header.Set("User-Agent", version.UserAgent)
	if len(req.Body) > 0 && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	res, err := hc.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return Response{}, err
	}
	return Response{Status: res.StatusCode, Body: b, FinalURL: rawURL, Header: res.Header}, nil
}

// Ping checks that base URL answers HTTP. Prefixes are not applied.
func (c *Client) Ping(ctx context.Context) (int, error) {
	raw := strings.TrimRight(c.BaseURL, "/") + "/"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("User-Agent", version.UserAgent)
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	res, err := hc.Do(httpReq)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	return res.StatusCode, nil
}

func (c *Client) candidateURLs(req Request) ([]string, error) {
	return c.urls(req, req.Query)
}

func (c *Client) urls(req Request, q url.Values) ([]string, error) {
	path := req.Path
	if path == "" {
		path = "/"
	}
	if PathAlreadyPrefixed(path) {
		raw, err := JoinURL(c.BaseURL, "", path, q)
		if err != nil {
			return nil, err
		}
		return []string{raw}, nil
	}
	prefixes := c.Prefixes
	if len(prefixes) == 0 {
		prefixes = []string{""}
	}
	out := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		raw, err := JoinURL(c.BaseURL, p, path, q)
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}

// PreviewURLs is candidateURLs with adminPassWord masked. Used by dry-run.
func (c *Client) PreviewURLs(req Request) ([]string, error) {
	return c.urls(req, RedactQuery(req.Query))
}

// NormalizeMethod uppercases and validates a method token.
func NormalizeMethod(method string) (string, error) {
	m := strings.ToUpper(strings.TrimSpace(method))
	switch m {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead:
		return m, nil
	default:
		return "", fmt.Errorf("unsupported method %q", method)
	}
}

// EnsureLeadingSlash makes api paths absolute.
func EnsureLeadingSlash(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}
