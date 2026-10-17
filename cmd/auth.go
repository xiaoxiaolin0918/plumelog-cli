package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"plumelog-cli/internal/client"
	"plumelog-cli/internal/config"
	"plumelog-cli/internal/output"
	"plumelog-cli/internal/risk"
)

func (a *App) newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Session login, logout, and local cookie status",
		RunE:  helpOnly,
	}
	cmd.AddCommand(a.newLoginCmd(), a.newLogoutCmd(), a.newStatusCmd())
	return cmd
}

func (a *App) newLoginCmd() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:  "login",
		Long: "POST /login (and /plumelog/login when --path-prefix auto falls back).\nBody is {\"username\",\"password\"}. A successful call stores Set-Cookie in the jar.\nPassword is read from --password, PLUMELOG_LOGIN_PASSWORD, or a terminal prompt.\nAn empty username still POSTs /login so an open server (no loginUsername) can mint a session.",
		Example: `  plumelog auth login --username admin --password "$PLUMELOG_LOGIN_PASSWORD"
  plumelog auth login`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			user := username
			if user == "" {
				user = os.Getenv("PLUMELOG_LOGIN_USERNAME")
			}
			if user == "" {
				user = a.cfg.LoginUsername
			}
			pass := password
			if pass == "" {
				pass = os.Getenv("PLUMELOG_LOGIN_PASSWORD")
			}
			if pass == "" && user != "" {
				var err error
				pass, err = promptPassword(cmd)
				if err != nil {
					return a.fail(cmd, risk.Write, err)
				}
			}
			a.track(pass)
			body, err := json.Marshal(map[string]string{"username": user, "password": pass})
			if err != nil {
				return a.fail(cmd, risk.Write, err)
			}
			req := client.Request{Method: "POST", Path: "/login", Body: body}
			stop, err := a.prepare(cmd, risk.Mutation(), &req)
			if stop || err != nil {
				return err
			}
			resp, err := a.do(cmd, req)
			if err != nil {
				return err
			}
			if code, ok := client.ResultCode(resp.Body); ok && code == 402 {
				_ = a.jar.Clear()
				return a.fail(cmd, risk.Write, errors.New("invalid login (402)"))
			}
			if code, ok := client.ResultCode(resp.Body); ok && code >= 400 {
				return a.fail(cmd, risk.Write, fmt.Errorf("server returned code %d", code))
			}
			if resp.Status >= 400 {
				return a.fail(cmd, risk.Write, fmt.Errorf("HTTP %d", resp.Status))
			}
			authMode := "session"
			if user == "" {
				authMode = "open"
			}
			a.cfg.BaseURL = a.baseURL
			if user != "" {
				a.cfg.LoginUsername = user
			}
			hint := ""
			if err := config.Save(a.configPath, a.cfg); err != nil {
				hint = "session cookie saved; config file was not updated"
			}
			env := output.Envelope{
				OK: true,
				Result: output.MustResult(map[string]any{
					"base_url":    a.baseURL,
					"cookie_path": a.cookiePath,
					"auth":        authMode,
					"has_cookie":  a.jar.HasAny(),
				}),
			}
			if hint != "" {
				env.Hint = hint
			}
			return a.write(cmd, env)
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "login username (env PLUMELOG_LOGIN_USERNAME)")
	cmd.Flags().StringVar(&password, "password", "", "login password (env PLUMELOG_LOGIN_PASSWORD); never printed")
	mark(cmd, risk.Write, "Log in and persist the session cookie")
	return cmd
}

func promptPassword(cmd *cobra.Command) (string, error) {
	in := cmd.InOrStdin()
	f, ok := in.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return "", errors.New("password required: pass --password or set PLUMELOG_LOGIN_PASSWORD")
	}
	fmt.Fprint(cmd.ErrOrStderr(), "Password: ")
	b, err := term.ReadPassword(int(f.Fd()))
	fmt.Fprintln(cmd.ErrOrStderr())
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", errors.New("password required: pass --password or set PLUMELOG_LOGIN_PASSWORD")
	}
	return string(b), nil
}

func (a *App) newLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "logout",
		Long: "POST /logout and clear the local cookie jar even if the server is unreachable.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := client.Request{Method: "POST", Path: "/logout"}
			stop, err := a.prepare(cmd, risk.Mutation(), &req)
			if stop || err != nil {
				return err
			}
			resp, callErr := a.client.Do(cmd.Context(), req)
			cleared := true
			if err := a.jar.Clear(); err != nil {
				cleared = false
				callErr = err
			}
			serverOK := callErr == nil && resp.Status < 400
			if code, ok := client.ResultCode(resp.Body); ok && code >= 400 {
				serverOK = false
			}
			result := map[string]any{
				"base_url":          a.baseURL,
				"cookie_path":       a.cookiePath,
				"local_jar_cleared": cleared,
				"server_logout":     serverOK,
			}
			if serverOK && cleared {
				return a.ok(cmd, result)
			}
			env := output.Envelope{
				OK:     false,
				Error:  output.ErrorBody{Type: "cli", Message: "server logout failed; local cookie jar clear attempted"},
				Result: output.MustResult(result),
			}
			if err := a.write(cmd, env); err != nil {
				return err
			}
			return &PrintedError{Msg: output.ErrorMessage(env.Error)}
		},
	}
	mark(cmd, risk.Write, "Log out and clear the local cookie jar")
	return cmd
}

func (a *App) newStatusCmd() *cobra.Command {
	var probe bool
	cmd := &cobra.Command{
		Use:  "status",
		Long: "Report has_cookie, cookie_path, and base_url. Cookie values are never printed.\n--probe sends a read-only POST /getRunModel and reports whether the session is accepted.",
		Example: `  plumelog auth status
  plumelog auth status --probe`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result := map[string]any{
				"has_cookie":  a.jar != nil && a.jar.HasAny(),
				"cookie_path": a.cookiePath,
				"base_url":    a.baseURL,
			}
			if !probe {
				return a.ok(cmd, result)
			}
			req := client.Request{Method: "POST", Path: "/getRunModel"}
			resp, err := a.client.Do(cmd.Context(), req)
			probeOut := map[string]any{"endpoint": "/getRunModel"}
			if err != nil {
				probeOut["ok"] = false
				probeOut["error"] = "request failed"
				result["probe"] = probeOut
				env := output.Envelope{OK: false, Error: output.ErrorBody{Type: "cli", Message: "probe failed"}, Result: output.MustResult(result)}
				if werr := a.write(cmd, env); werr != nil {
					return werr
				}
				return &PrintedError{Msg: output.ErrorMessage(env.Error)}
			}
			if code, ok := client.ResultCode(resp.Body); ok && code == 401 {
				probeOut["ok"] = true
				probeOut["authenticated"] = false
				result["probe"] = probeOut
				return a.ok(cmd, result)
			}
			if resp.Status >= 400 {
				probeOut["ok"] = false
				probeOut["authenticated"] = false
				probeOut["http_status"] = resp.Status
				result["probe"] = probeOut
				env := output.Envelope{OK: false, Error: output.ErrorBody{Type: "cli", Message: "probe failed"}, Result: output.MustResult(result)}
				if werr := a.write(cmd, env); werr != nil {
					return werr
				}
				return &PrintedError{Msg: output.ErrorMessage(env.Error)}
			}
			probeOut["ok"] = true
			probeOut["authenticated"] = true
			if raw := output.RawResult(resp.Body); raw != nil {
				var v any
				if json.Unmarshal(raw, &v) == nil {
					probeOut["run_model"] = v
				}
			}
			result["probe"] = probeOut
			return a.ok(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&probe, "probe", false, "POST /getRunModel to see if the session is accepted")
	mark(cmd, risk.Read, "Show cookie presence without printing cookie values")
	return cmd
}
