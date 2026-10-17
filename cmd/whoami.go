package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"plumelog-cli/internal/client"
	"plumelog-cli/internal/output"
	"plumelog-cli/internal/risk"
)

func (a *App) newWhoamiCmd() *cobra.Command {
	var probe bool
	cmd := &cobra.Command{
		Use: "whoami",
		Long: `Alias of auth status with a stable field set for agents.

Always prints (never secrets):
  base_url, authenticated, has_cookie, cookie_path, path_prefix

Cookie values and passwords are never printed. With --probe, POSTs /getRunModel
to refine authenticated from the server response.`,
		Example: `  plumelog whoami
  plumelog whoami --probe`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			hasCookie := a.jar != nil && a.jar.HasAny()
			result := map[string]any{
				"base_url":      a.baseURL,
				"authenticated": hasCookie,
				"has_cookie":    hasCookie,
				"cookie_path":   a.cookiePath,
				"path_prefix":   pathPrefixLabel(a.prefixMode),
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
				result["authenticated"] = false
				env := output.Envelope{
					OK:     false,
					Error:  output.ErrorBody{Type: "cli", Message: "probe failed", Hint: "check base_url and network"},
					Result: output.MustResult(result),
				}
				if werr := a.write(cmd, env); werr != nil {
					return werr
				}
				return &PrintedError{Msg: "probe failed"}
			}
			if code, ok := client.ResultCode(resp.Body); ok && code == 401 {
				probeOut["ok"] = true
				probeOut["authenticated"] = false
				result["probe"] = probeOut
				result["authenticated"] = false
				return a.ok(cmd, result)
			}
			if resp.Status >= 400 {
				probeOut["ok"] = false
				probeOut["authenticated"] = false
				probeOut["http_status"] = resp.Status
				result["probe"] = probeOut
				result["authenticated"] = false
				env := output.Envelope{
					OK:     false,
					Error:  output.ErrorBody{Type: "api", Message: "probe failed", Hint: "check base_url and session"},
					Result: output.MustResult(result),
				}
				if werr := a.write(cmd, env); werr != nil {
					return werr
				}
				return &PrintedError{Msg: "probe failed"}
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
			result["authenticated"] = true
			return a.ok(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&probe, "probe", false, "POST /getRunModel to see if the session is accepted")
	mark(cmd, risk.Read, "Show session identity metadata without secrets")
	return cmd
}

func pathPrefixLabel(mode string) string {
	if mode == "" {
		return "root"
	}
	return mode
}
