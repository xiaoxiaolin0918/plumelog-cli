package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xiaoxiaolin0918/plumelog-cli/internal/client"
	"github.com/xiaoxiaolin0918/plumelog-cli/internal/output"
	"github.com/xiaoxiaolin0918/plumelog-cli/internal/risk"
)

func (a *App) newTraceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace",
		Short: "Look up a trace by traceId",
		RunE:  helpOnly,
	}
	cmd.AddCommand(a.newTraceGetCmd())
	return cmd
}

func (a *App) newTraceGetCmd() *cobra.Command {
	var f searchFlags
	cmd := &cobra.Command{
		Use: "get",
		Long: `POST /clientQuery?trace=true with a match_phrase on traceId.
Sort is dtTime asc, seq asc. The default page size is 1000.
Trace indices look like plume_log_trace_YYYYMMDD (selected via clientStartDate/clientEndDate).
Run-log indices look like plume_log_run_YYYYMMDD — same traceId may exist there even when
the trace index is empty (e.g. some test envs). If this command returns 0 hits, try:
  plumelog query search --trace-id <id>
(or check the envelope hint).`,
		Example: `  plumelog trace get --trace-id TRACE_ID
  plumelog trace get --trace-id TRACE_ID --from-time 1700000000000 --to-time 1700003600000
  plumelog query search --trace-id TRACE_ID`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if f.traceID == "" {
				return a.fail(cmd, risk.Read, errors.New("--trace-id is required"))
			}
			req, err := a.planSearch(cmd, f, true, 1000)
			if err != nil {
				return a.fail(cmd, risk.Read, err)
			}
			return a.invokeTraceGet(cmd, req, f.traceID)
		},
	}
	cmd.Flags().StringVar(&f.traceID, "trace-id", "", "traceId to load (required)")
	cmd.Flags().StringVar(&f.app, "app", "", "optional appName match_phrase")
	cmd.Flags().StringVar(&f.fromTime, "from-time", "", "clientStartDate")
	cmd.Flags().StringVar(&f.toTime, "to-time", "", "clientEndDate")
	cmd.Flags().IntVar(&f.size, "size", 0, "page size (default 1000)")
	cmd.Flags().IntVar(&f.from, "from", 0, "page offset")
	cmd.Flags().BoolVar(&f.noHighlight, "no-highlight", false, "omit the content highlight block")
	mark(cmd, risk.Read, "Fetch one trace from the trace indices")
	return cmd
}

// invokeTraceGet is invoke + empty-hit hint (Issue #21). Prefer hint over auto-fallback.
func (a *App) invokeTraceGet(cmd *cobra.Command, req client.Request, traceID string) error {
	stop, err := a.prepare(cmd, risk.ReadOnly(), &req)
	if stop || err != nil {
		return err
	}
	resp, err := a.do(cmd, req)
	if err != nil {
		return err
	}
	if a.autoLogin && !a.dryRun && isUnauthorized(resp) && !isLoginRequest(req) {
		if err := a.silentLogin(cmd); err != nil {
			return err
		}
		resp, err = a.do(cmd, req)
		if err != nil {
			return err
		}
	}
	return a.finishTraceGet(cmd, resp, traceID)
}

func (a *App) finishTraceGet(cmd *cobra.Command, resp client.Response, traceID string) error {
	// Reuse failure paths from finishUpstream for auth/HTTP errors.
	if code, ok := client.ResultCode(resp.Body); ok && code >= 400 {
		return a.finishUpstream(cmd, resp)
	}
	if resp.Status >= 400 {
		return a.finishUpstream(cmd, resp)
	}
	if len(strings.TrimSpace(string(resp.Body))) == 0 {
		return a.finishUpstream(cmd, resp)
	}
	env := output.Envelope{
		OK:     true,
		URL:    client.RedactURL(resp.FinalURL),
		Result: output.RawResult(resp.Body),
	}
	if total, ok := esHitsTotal(resp.Body); ok && total == 0 {
		id := strings.TrimSpace(traceID)
		env.Hint = fmt.Sprintf(
			"trace index returned 0 hits for %s; try run-log search: plumelog query search --trace-id %s",
			id, id,
		)
	}
	return a.write(cmd, env)
}

// esHitsTotal reads Elasticsearch hits.total (number or {value,relation}).
func esHitsTotal(body []byte) (int64, bool) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return 0, false
	}
	hits, ok := root["hits"].(map[string]any)
	if !ok {
		return 0, false
	}
	switch v := hits["total"].(type) {
	case float64:
		return int64(v), true
	case map[string]any:
		switch n := v["value"].(type) {
		case float64:
			return int64(n), true
		default:
			return 0, false
		}
	default:
		return 0, false
	}
}
