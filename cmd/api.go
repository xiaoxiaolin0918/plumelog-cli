package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"plumelog-cli/internal/client"
	"plumelog-cli/internal/risk"
)

func (a *App) newAPICmd() *cobra.Command {
	var body, bodyFile string
	var query, headers []string
	cmd := &cobra.Command{
		Use:   "api <METHOD> <path>",
		Short: "Call any PlumeLog HTTP path (Risk: varies)",
		Long: `Escape hatch. path is relative to --base-url and --path-prefix unless it already
starts with /plumelog/. A query string on the path is kept and merged with --query.

Known destructive paths (deleteQueue, deleteIndex, resetIndex, deleteByQuery,
deleteWarningRule, delExtendfield) are high-risk-write and require --yes.
deleteQueue and deleteIndex also require --admin-password, which is sent as
adminPassWord and redacted in --dry-run.

Non-POST methods are not checked by the login interceptor, but the cookie jar
is still attached.`,
		Example: `  plumelog api POST /getRunModel
  plumelog api POST /clientQuery --query size=20 --query from=0 --body-file query.json
  plumelog api POST /deleteQueue --dry-run`,
		Args: cobra.ExactArgs(2),
		Annotations: map[string]string{
			"risk": "varies",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			method, err := client.NormalizeMethod(args[0])
			if err != nil {
				return a.fail(cmd, "", err)
			}
			path, q, err := splitPathQuery(args[1])
			if err != nil {
				return a.fail(cmd, "", err)
			}
			for _, item := range query {
				k, v, ok := strings.Cut(item, "=")
				if !ok || strings.TrimSpace(k) == "" {
					return a.fail(cmd, "", fmt.Errorf("--query %q: want key=value", item))
				}
				q.Set(strings.TrimSpace(k), v)
			}
			hdr := map[string]string{}
			for _, item := range headers {
				k, v, ok := strings.Cut(item, ":")
				if !ok || strings.TrimSpace(k) == "" {
					return a.fail(cmd, "", fmt.Errorf("--header %q: want Name: value", item))
				}
				hdr[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
			var b []byte
			switch {
			case body != "" && bodyFile != "":
				return a.fail(cmd, "", errors.New("pass only one of --body and --body-file"))
			case bodyFile != "":
				b, err = os.ReadFile(bodyFile)
				if err != nil {
					return a.fail(cmd, "", err)
				}
			case body != "":
				b = []byte(body)
			}
			if len(bytes.TrimSpace(b)) > 0 && json.Valid(bytes.TrimSpace(b)) {
				b = bytes.TrimSpace(b)
			}
			policy := risk.ForAPI(method, path)
			req := client.Request{Method: method, Path: path, Query: q, Body: b, Headers: hdr}
			return a.invoke(cmd, policy, req)
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "raw request body")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "file to use as the request body")
	cmd.Flags().StringArrayVar(&query, "query", nil, "repeatable query key=value (overrides the same key on the path)")
	cmd.Flags().StringArrayVar(&headers, "header", nil, "repeatable header Name: value (Cookie and Authorization are redacted in dry-run)")
	return cmd
}

func splitPathQuery(raw string) (string, url.Values, error) {
	raw = strings.TrimSpace(raw)
	q := url.Values{}
	path := raw
	if i := strings.Index(raw, "?"); i >= 0 {
		parsed, err := url.ParseQuery(raw[i+1:])
		if err != nil {
			return "", nil, err
		}
		q = parsed
		path = raw[:i]
	}
	return client.EnsureLeadingSlash(path), q, nil
}
