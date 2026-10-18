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
			_, _ = io.WriteString(w, webLoginFormHTML(a.baseURL, ""))
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad form", http.StatusBadRequest)
				return
			}
			user := strings.TrimSpace(r.Form.Get("username"))
			pass := r.Form.Get("password")
			rawBase := r.Form.Get("base_url")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			base, baseErr := normalizeWebBaseURL(rawBase)
			if baseErr != nil {
				// Keep listener open so the user can correct the URL.
				_, _ = io.WriteString(w, webLoginFormHTML(strings.TrimSpace(rawBase), baseErr.Error()))
				return
			}
			// Form value wins over flag/env/config prefill.
			a.applyWebBaseURL(base)
			// Never log or write password.
			loginErr := a.persistLoginFromWeb(cmd, user, pass)
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

// normalizeWebBaseURL mirrors resolveInto / config set rules for base_url.
func normalizeWebBaseURL(raw string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	if base == "" {
		return "", errors.New("服务地址不能为空")
	}
	if !strings.Contains(base, "://") {
		return "", errors.New("服务地址格式无效，需类似 http://localhost:8891")
	}
	return base, nil
}

// applyWebBaseURL updates the in-memory client target used by the subsequent login POST.
func (a *App) applyWebBaseURL(base string) {
	a.baseURL = base
	if a.client != nil {
		a.client.BaseURL = base
	}
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

func webLoginSharedCSS() string {
	return `
:root{--bg:#f0f2f5;--card:#fff;--text:#1a1a1a;--muted:#667085;--border:#e4e7ec;--accent:#2563eb;--accent-hover:#1d4ed8;--ok:#059669;--fail:#dc2626;--shadow:0 8px 24px rgba(16,24,40,.08)}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;padding:1.5rem;font-family:system-ui,-apple-system,"Segoe UI","PingFang SC","Hiragino Sans GB","Microsoft YaHei",sans-serif;background:linear-gradient(160deg,#eef2ff 0%,var(--bg) 45%,#f8fafc 100%);color:var(--text);line-height:1.5}
.card{width:100%;max-width:26rem;background:var(--card);border:1px solid var(--border);border-radius:14px;box-shadow:var(--shadow);padding:1.75rem 1.75rem 1.5rem}
.brand{font-size:.8rem;font-weight:600;letter-spacing:.04em;text-transform:uppercase;color:var(--accent);margin:0 0 .35rem}
h1{font-size:1.35rem;font-weight:650;margin:0 0 .75rem}
.note{color:var(--muted);font-size:.9rem;margin:0 0 1.25rem}
label{display:block;font-size:.875rem;font-weight:550;margin:0 0 .35rem;color:#344054}
.field{margin:0 0 1rem}
input{width:100%;padding:.65rem .75rem;border:1px solid var(--border);border-radius:8px;font:inherit;background:#fff;color:var(--text)}
input:focus{outline:2px solid rgba(37,99,235,.35);border-color:var(--accent)}
button{display:inline-flex;align-items:center;justify-content:center;width:100%;margin-top:.35rem;padding:.7rem 1rem;border:none;border-radius:8px;font:inherit;font-weight:600;cursor:pointer;background:var(--accent);color:#fff}
button:hover{background:var(--accent-hover)}
.meta{list-style:none;padding:0;margin:1rem 0 0;font-size:.9rem}
.meta li{margin:.4rem 0;padding:.55rem .7rem;background:#f9fafb;border:1px solid var(--border);border-radius:8px;word-break:break-all}
.meta code{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:.85em}
.status{display:inline-block;font-size:.75rem;font-weight:650;padding:.2rem .55rem;border-radius:999px;margin-bottom:.65rem}
.status.ok{background:#ecfdf5;color:var(--ok)}
.status.fail{background:#fef2f2;color:var(--fail)}
.err{margin:0 0 1rem;padding:.75rem .85rem;background:#fef2f2;border:1px solid #fecaca;border-radius:8px;color:#7f1d1d;font-size:.9rem;word-break:break-word}
.foot{margin:1.1rem 0 0;color:var(--muted);font-size:.85rem}
`
}

func webLoginFormHTML(baseURL, errMsg string) string {
	errBlock := ""
	if errMsg != "" {
		errBlock = `<p class="err">` + html.EscapeString(errMsg) + `</p>`
	}
	return `<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>plumelog 登录</title>
<style>` + webLoginSharedCSS() + `</style></head><body>
<main class="card">
<p class="brand">plumelog CLI</p>
<h1>本机登录</h1>
<p class="note">凭据仅提交到本机 127.0.0.1，不会离开此电脑。密码不会再次显示，也不会写入日志。服务地址用于连接 PlumeLog；提交值优先于 CLI 预填。</p>
` + errBlock + `
<form method="POST" autocomplete="off">
<div class="field"><label for="base_url">服务地址</label><input id="base_url" name="base_url" type="url" value="` + html.EscapeString(baseURL) + `" placeholder="http://localhost:8891" autocomplete="url"></div>
<div class="field"><label for="username">用户名</label><input id="username" name="username" autocomplete="username" autofocus></div>
<div class="field"><label for="password">密码</label><input id="password" name="password" type="password" autocomplete="current-password"></div>
<button type="submit">登录</button>
</form>
<p class="foot">此页面由 <code>plumelog auth login --web</code> 临时打开，完成后可关闭标签页。</p>
</main></body></html>`
}
func webLoginOKHTML(baseURL, cookiePath string) string {
	return fmt.Sprintf(`<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>登录成功</title>
<style>`+webLoginSharedCSS()+`</style></head><body>
<main class="card">
<span class="status ok">成功</span>
<h1>登录成功</h1>
<p class="note">会话 Cookie 已写入本机。你可以关闭此标签页，回到终端继续使用 CLI。</p>
<ul class="meta">
<li>已登录到 base_url：<code>%s</code></li>
<li>cookie_path：<code>%s</code></li>
</ul>
</main></body></html>`, html.EscapeString(baseURL), html.EscapeString(cookiePath))
}

func webLoginFailHTML(err error) string {
	msg := "未知错误"
	if err != nil {
		msg = webLoginErrZh(err)
	}
	return fmt.Sprintf(`<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>登录失败</title>
<style>`+webLoginSharedCSS()+`</style></head><body>
<main class="card">
<span class="status fail">失败</span>
<h1>登录失败</h1>
<p class="err">%s</p>
<p class="foot">请关闭此标签页，在终端用 <code>plumelog auth login --web</code> 重试（如需覆盖已有会话请加 <code>--force</code>）。</p>
</main></body></html>`, msg)
}

// webLoginErrZh maps common login errors to readable Chinese; unknown errors are HTML-escaped as-is.
func webLoginErrZh(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "invalid login"):
		return "用户名或密码错误"
	case strings.HasPrefix(s, "HTTP "):
		return "服务器 HTTP 错误：" + html.EscapeString(s)
	case strings.Contains(s, "server returned code"):
		return "服务器返回错误码：" + html.EscapeString(s)
	default:
		return html.EscapeString(s)
	}
}
