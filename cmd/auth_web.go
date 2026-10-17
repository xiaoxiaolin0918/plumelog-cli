package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/xiaoxiaolin0918/plumelog-cli/internal/client"
	"github.com/xiaoxiaolin0918/plumelog-cli/internal/config"
	"github.com/xiaoxiaolin0918/plumelog-cli/internal/output"
	"github.com/xiaoxiaolin0918/plumelog-cli/internal/risk"
)

const webLoginTimeout = 120 * time.Second

// webLoginDeps lets tests stub the browser opener and/or capture the listen URL.
type webLoginDeps struct {
	Timeout     time.Duration
	OpenBrowser func(u string) error
	// OnReady is called once the listener is up with the one-time URL (tests only).
	OnReady func(u string)
}

func (a *App) runWebLogin(cmd *cobra.Command, force bool, deps webLoginDeps) error {
	if a.jar != nil && a.jar.HasAny() && !force {
		return a.fail(cmd, risk.Write, errors.New("cookie jar already has a session; pass --force to replace it"))
	}
	if a.dryRun {
		env := output.Envelope{
			OK:     true,
			DryRun: true,
			Result: output.MustResult(map[string]any{
				"web_login": true,
				"bind":      "127.0.0.1",
				"timeout_s": int(webLoginTimeout.Seconds()),
				"note":      "would open a local browser form; no password is printed",
			}),
		}
		return a.write(cmd, env)
	}

	timeout := deps.Timeout
	if timeout <= 0 {
		timeout = webLoginTimeout
	}
	open := deps.OpenBrowser
	if open == nil {
		open = openSystemBrowser
	}

	token, err := randomToken(16)
	if err != nil {
		return a.fail(cmd, risk.Write, err)
	}

	// Explicitly bind loopback only — never 0.0.0.0.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return a.fail(cmd, risk.Write, fmt.Errorf("bind 127.0.0.1: %w", err))
	}
	addr := ln.Addr().String()
	if host, _, splitErr := net.SplitHostPort(addr); splitErr == nil {
		if host != "127.0.0.1" && host != "::1" {
			_ = ln.Close()
			return a.fail(cmd, risk.Write, fmt.Errorf("refusing non-loopback bind %s", host))
		}
	}

	formPath := "/cli-login/" + token + "/"
	formURL := "http://" + addr + formPath

	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()

	type outcome struct {
		user string
		pass string
		err  error
	}
	done := make(chan outcome, 1)
	var once sync.Once
	finish := func(o outcome) {
		once.Do(func() { done <- o })
	}

	mux := http.NewServeMux()
	mux.HandleFunc(formPath, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = io.WriteString(w, webLoginFormHTML())
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad form", http.StatusBadRequest)
				return
			}
			user := strings.TrimSpace(r.Form.Get("username"))
			pass := r.Form.Get("password")
			// Never log or write password.
			loginErr := a.persistLoginFromWeb(cmd, user, pass)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			if loginErr != nil {
				_, _ = io.WriteString(w, webLoginFailHTML(loginErr))
				finish(outcome{err: loginErr})
				return
			}
			_, _ = io.WriteString(w, webLoginOKHTML(a.baseURL, a.cookiePath))
			finish(outcome{user: user, pass: pass})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	go func() {
		_ = srv.Serve(ln)
	}()
	defer func() {
		shutdownCtx, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		_ = srv.Shutdown(shutdownCtx)
		_ = ln.Close()
	}()

	fmt.Fprintf(cmd.ErrOrStderr(), "web login: open %s (timeout %s)\n", formURL, timeout)
	if deps.OnReady != nil {
		deps.OnReady(formURL)
	}
	if err := open(formURL); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: could not open browser automatically: %v\n", err)
	}

	select {
	case o := <-done:
		if o.err != nil {
			return a.fail(cmd, risk.Write, o.err)
		}
		authMode := "session"
		if o.user == "" {
			authMode = "open"
		}
		return a.write(cmd, output.Envelope{
			OK: true,
			Result: output.MustResult(map[string]any{
				"base_url":    a.baseURL,
				"cookie_path": a.cookiePath,
				"auth":        authMode,
				"has_cookie":  a.jar != nil && a.jar.HasAny(),
				"via":         "web",
			}),
		})
	case <-ctx.Done():
		return a.fail(cmd, risk.Write, errors.New("web login timed out; local listener closed"))
	}
}

// persistLoginFromWeb POSTs /login and updates config. Password is tracked for scrubbing only.
func (a *App) persistLoginFromWeb(cmd *cobra.Command, user, pass string) error {
	a.track(pass)
	body, err := json.Marshal(map[string]string{"username": user, "password": pass})
	if err != nil {
		return err
	}
	req := client.Request{Method: "POST", Path: "/login", Body: body}
	resp, err := a.do(cmd, req)
	if err != nil {
		return err
	}
	if code, ok := client.ResultCode(resp.Body); ok && code == 402 {
		_ = a.jar.Clear()
		return errors.New("invalid login (402)")
	}
	if code, ok := client.ResultCode(resp.Body); ok && code >= 400 {
		return fmt.Errorf("server returned code %d", code)
	}
	if resp.Status >= 400 {
		return fmt.Errorf("HTTP %d", resp.Status)
	}
	a.cfg.BaseURL = a.baseURL
	if user != "" {
		a.cfg.LoginUsername = user
	}
	_ = config.Save(a.configPath, a.cfg)
	return nil
}

func randomToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func openSystemBrowser(u string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		c = exec.Command("open", u)
	default:
		c = exec.Command("xdg-open", u)
	}
	return c.Start()
}

func webLoginFormHTML() string {
	return `<!DOCTYPE html><html><head><meta charset="utf-8"><title>plumelog login</title>
<style>body{font-family:system-ui,sans-serif;max-width:24rem;margin:3rem auto;padding:0 1rem}
label{display:block;margin:.75rem 0 .25rem}input{width:100%;padding:.4rem;box-sizing:border-box}
button{margin-top:1rem;padding:.5rem 1rem}p.note{color:#555;font-size:.9rem}</style></head><body>
<h1>plumelog login</h1>
<p class="note">Credentials stay on this machine (127.0.0.1). The password is never shown again.</p>
<form method="POST" autocomplete="off">
<label>Username</label><input name="username" autocomplete="username">
<label>Password</label><input name="password" type="password" autocomplete="current-password">
<button type="submit">Sign in</button>
</form></body></html>`
}

func webLoginOKHTML(baseURL, cookiePath string) string {
	return fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>plumelog login ok</title>
<style>body{font-family:system-ui,sans-serif;max-width:28rem;margin:3rem auto;padding:0 1rem}
code{word-break:break-all}</style></head><body>
<h1>Signed in</h1>
<p>You can close this tab.</p>
<ul>
<li>base_url: <code>%s</code></li>
<li>cookie_path: <code>%s</code></li>
</ul>
</body></html>`, html.EscapeString(baseURL), html.EscapeString(cookiePath))
}

func webLoginFailHTML(err error) string {
	msg := "login failed"
	if err != nil {
		// Scrub is applied by callers via track; still avoid echoing raw error if it somehow embeds secrets.
		msg = html.EscapeString(err.Error())
	}
	return fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>plumelog login failed</title>
<style>body{font-family:system-ui,sans-serif;max-width:28rem;margin:3rem auto;padding:0 1rem}</style></head><body>
<h1>Login failed</h1><p>%s</p>
<p>You can close this tab and retry from the CLI.</p>
</body></html>`, msg)
}
