// Package client is the PlumeLog HTTP session client.
package client

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Jar is a Netscape cookies.txt store (curl-compatible) that implements
// http.CookieJar. Cookie values are persisted at mode 0600 and are never
// printed by status or doctor.
type Jar struct {
	mu      sync.Mutex
	path    string
	entries []entry
}

type entry struct {
	Domain   string
	HostOnly bool
	Path     string
	Secure   bool
	Expires  time.Time // zero means a session cookie, kept until logout
	HTTPOnly bool
	Name     string
	Value    string
}

// Open loads a Netscape cookie file. A missing file yields an empty jar.
func Open(path string) (*Jar, error) {
	j := &Jar{path: path}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return j, nil
		}
		return nil, err
	}
	if err := j.parse(b); err != nil {
		return nil, fmt.Errorf("cookie jar %s: %w", path, err)
	}
	return j, nil
}

// Path is the on-disk file.
func (j *Jar) Path() string {
	if j == nil {
		return ""
	}
	return j.path
}

// HasAny reports whether the jar holds at least one unexpired cookie.
func (j *Jar) HasAny() bool {
	if j == nil {
		return false
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	now := time.Now()
	for _, e := range j.entries {
		if e.Expires.IsZero() || e.Expires.After(now) {
			return true
		}
	}
	return false
}

// Names returns cookie names that would be sent to u. Values are omitted.
func (j *Jar) Names(u *url.URL) []string {
	cookies := j.Cookies(u)
	names := make([]string, 0, len(cookies))
	seen := map[string]struct{}{}
	for _, c := range cookies {
		if _, ok := seen[c.Name]; ok {
			continue
		}
		seen[c.Name] = struct{}{}
		names = append(names, c.Name)
	}
	return names
}

// SecretValues returns cookie values so the CLI can scrub them from stdout.
// Callers must not print the result.
func (j *Jar) SecretValues() []string {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]string, 0, len(j.entries))
	for _, e := range j.entries {
		if e.Value != "" {
			out = append(out, e.Value)
		}
	}
	return out
}

// Clear drops every cookie and rewrites the file as a header-only jar.
func (j *Jar) Clear() error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	j.entries = nil
	j.mu.Unlock()
	return j.Save()
}

// SetCookies implements http.CookieJar.
func (j *Jar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if j == nil || u == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	host := u.Hostname()
	for _, c := range cookies {
		if c == nil || c.Name == "" {
			continue
		}
		e := entry{
			Domain:   host,
			HostOnly: true,
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HttpOnly,
			Name:     c.Name,
			Value:    c.Value,
			Expires:  c.Expires,
		}
		if c.Domain != "" {
			e.HostOnly = false
			e.Domain = strings.TrimPrefix(c.Domain, ".")
		}
		if e.Path == "" {
			e.Path = "/"
		}
		if c.MaxAge < 0 || (!c.Expires.IsZero() && c.Expires.Before(time.Now())) {
			j.deleteLocked(e)
			continue
		}
		j.upsertLocked(e)
	}
	_ = j.saveLocked()
}

// Cookies implements http.CookieJar.
func (j *Jar) Cookies(u *url.URL) []*http.Cookie {
	if j == nil || u == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	now := time.Now()
	host := u.Hostname()
	var out []*http.Cookie
	for _, e := range j.entries {
		if !e.Expires.IsZero() && !e.Expires.After(now) {
			continue
		}
		if !domainMatch(host, e) {
			continue
		}
		if !pathMatch(u.Path, e.Path) {
			continue
		}
		if e.Secure && u.Scheme != "https" {
			continue
		}
		out = append(out, &http.Cookie{Name: e.Name, Value: e.Value})
	}
	return out
}

// Save writes the Netscape file at mode 0600.
func (j *Jar) Save() error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.saveLocked()
}

func (j *Jar) upsertLocked(e entry) {
	for i := range j.entries {
		if sameCookie(j.entries[i], e) {
			j.entries[i] = e
			return
		}
	}
	j.entries = append(j.entries, e)
}

func (j *Jar) deleteLocked(e entry) {
	kept := j.entries[:0]
	for _, cur := range j.entries {
		if sameCookie(cur, e) {
			continue
		}
		kept = append(kept, cur)
	}
	j.entries = kept
}

func sameCookie(a, b entry) bool {
	return a.Name == b.Name && a.Path == b.Path && a.Domain == b.Domain && a.HostOnly == b.HostOnly
}

func (j *Jar) saveLocked() error {
	if j.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(j.path), 0o700); err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString("# Netscape HTTP Cookie File\n")
	buf.WriteString("# https://curl.se/docs/http-cookies.html\n")
	buf.WriteString("# plumelog-cli session store. Mode 0600. Do not print or share values.\n")
	for _, e := range j.entries {
		exp := "0"
		if !e.Expires.IsZero() {
			exp = strconv.FormatInt(e.Expires.Unix(), 10)
		}
		domain := e.Domain
		if e.HTTPOnly {
			domain = "#HttpOnly_" + domain
		}
		sub := "FALSE"
		if !e.HostOnly {
			sub = "TRUE"
		}
		sec := "FALSE"
		if e.Secure {
			sec = "TRUE"
		}
		fmt.Fprintf(&buf, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", domain, sub, e.Path, sec, exp, e.Name, e.Value)
	}
	if err := os.WriteFile(j.path, buf.Bytes(), 0o600); err != nil {
		return err
	}
	return os.Chmod(j.path, 0o600)
}

func (j *Jar) parse(b []byte) error {
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := sc.Text()
		if line == "" || (strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "#HttpOnly_")) {
			continue
		}
		httpOnly := false
		if strings.HasPrefix(line, "#HttpOnly_") {
			httpOnly = true
			line = strings.TrimPrefix(line, "#HttpOnly_")
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 7 {
			return fmt.Errorf("bad cookie line")
		}
		expUnix, err := strconv.ParseInt(parts[4], 10, 64)
		if err != nil {
			return fmt.Errorf("cookie expiry: %w", err)
		}
		e := entry{
			Domain:   parts[0],
			HostOnly: !strings.EqualFold(parts[1], "TRUE"),
			Path:     parts[2],
			Secure:   strings.EqualFold(parts[3], "TRUE"),
			HTTPOnly: httpOnly,
			Name:     parts[5],
			Value:    parts[6],
		}
		if expUnix > 0 {
			e.Expires = time.Unix(expUnix, 0)
			if e.Expires.Before(time.Now()) {
				continue
			}
		}
		j.entries = append(j.entries, e)
	}
	return sc.Err()
}

func domainMatch(host string, e entry) bool {
	host = strings.TrimSuffix(host, ".")
	domain := strings.TrimSuffix(e.Domain, ".")
	if e.HostOnly {
		return strings.EqualFold(host, domain)
	}
	if strings.EqualFold(host, domain) {
		return true
	}
	return strings.HasSuffix(strings.ToLower(host), "."+strings.ToLower(domain))
}

func pathMatch(reqPath, cookiePath string) bool {
	if cookiePath == "" {
		cookiePath = "/"
	}
	if reqPath == "" {
		reqPath = "/"
	}
	if !strings.HasPrefix(reqPath, cookiePath) {
		return false
	}
	if len(reqPath) == len(cookiePath) || strings.HasSuffix(cookiePath, "/") {
		return true
	}
	return reqPath[len(cookiePath)] == '/'
}
