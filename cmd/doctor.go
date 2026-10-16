package cmd

import (
	"encoding/json"
	"strings"

	"github.com/spf13/cobra"

	"plumelog-cli/internal/client"
	"plumelog-cli/internal/output"
	"plumelog-cli/internal/risk"
)

func (a *App) newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "doctor",
		Long: `Check base URL reachability, cookie presence (names only), POST /getRunModel, and POST /getQueueCounts.
Prints config path, cookie path, and the resolved path prefix. Cookie values are never printed.`,
		Example: `  plumelog doctor
  plumelog doctor --base-url http://127.0.0.1:8891`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			type check struct {
				Name   string `json:"name"`
				OK     bool   `json:"ok"`
				Detail string `json:"detail,omitempty"`
			}
			checks := []check{}
			healthy := true

			status, err := a.client.Ping(cmd.Context())
			if err != nil {
				healthy = false
				checks = append(checks, check{Name: "reachable", OK: false, Detail: "connection failed"})
			} else {
				checks = append(checks, check{Name: "reachable", OK: true, Detail: "HTTP " + itoa(status)})
			}

			names := a.cookieNames()
			checks = append(checks, check{
				Name:   "cookie",
				OK:     true,
				Detail: cookieDetail(a.jar != nil && a.jar.HasAny(), names),
			})

			if healthy {
				_, ok, detail := a.probeJSON(cmd, "/getRunModel")
				if !ok {
					healthy = false
				}
				checks = append(checks, check{Name: "run_model", OK: ok, Detail: detail})
				_, qok, qdetail := a.probeJSON(cmd, "/getQueueCounts")
				if !qok {
					healthy = false
				}
				checks = append(checks, check{Name: "queue", OK: qok, Detail: qdetail})
			}

			result := map[string]any{
				"base_url":     a.baseURL,
				"config_path":  a.configPath,
				"cookie_path":  a.cookiePath,
				"path_prefix":  a.prefixMode,
				"prefix_order": a.client.Prefixes,
				"has_cookie":   a.jar != nil && a.jar.HasAny(),
				"cookie_names": names,
				"checks":       checks,
			}
			env := output.Envelope{OK: healthy, Result: output.MustResult(result)}
			if !healthy {
				env.Error = "doctor found a failing check"
				if err := a.write(cmd, env); err != nil {
					return err
				}
				return &PrintedError{Msg: output.ErrorMessage(env.Error)}
			}
			return a.write(cmd, env)
		},
	}
	mark(cmd, risk.Read, "Check connectivity, session, and run model")
	return cmd
}

func (a *App) probeJSON(cmd *cobra.Command, path string) (string, bool, string) {
	resp, err := a.client.Do(cmd.Context(), client.Request{Method: "POST", Path: path})
	if err != nil {
		return "", false, "request failed"
	}
	if code, ok := client.ResultCode(resp.Body); ok && code == 401 {
		return "", false, "unauthenticated (401); run plumelog auth login"
	}
	if code, ok := client.ResultCode(resp.Body); ok && code >= 400 {
		return "", false, "server code " + itoa(code)
	}
	if resp.Status >= 400 {
		return "", false, "HTTP " + itoa(resp.Status)
	}
	body := strings.TrimSpace(string(resp.Body))
	if body == "" {
		return "", true, "empty body"
	}
	if json.Valid([]byte(body)) {
		return body, true, compact(body)
	}
	return body, true, body
}

func cookieDetail(has bool, names []string) string {
	if !has {
		return "no cookie"
	}
	if len(names) == 0 {
		return "cookie present"
	}
	return "names=" + strings.Join(names, ",")
}

func compact(s string) string {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return s
	}
	if len(b) > 180 {
		return string(b[:180]) + "..."
	}
	return string(b)
}

func itoa(n int) string {
	return strconvItoa(n)
}

func strconvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
