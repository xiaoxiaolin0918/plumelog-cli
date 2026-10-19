package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xiaoxiaolin0918/plumelog-cli/internal/client"
	"github.com/xiaoxiaolin0918/plumelog-cli/internal/dsl"
	"github.com/xiaoxiaolin0918/plumelog-cli/internal/output"
	"github.com/xiaoxiaolin0918/plumelog-cli/internal/risk"
)

type appsFlags struct {
	source  string
	appLike string
	sample  string
	enrich  bool
}

func (a *App) newAppsCmd(use string) *cobra.Command {
	var f appsFlags
	cmd := &cobra.Command{
		Use: use,
		Long: `List application names.

cache (default) → POST /getAppNames
es → POST /queryAppNames (terms agg on appNameWithEnv)

Discovery helpers (Issue #20):
  --app-like SUBSTR   keep names containing SUBSTR (case-insensitive)
  --sample APP        preview recent run-log hits for one app (className/content)
  --enrich / --stats  opt-in per-app last-log-time + top className via clientQuery (N+1; capped)

Default list stays a fast single request; enrichment is off unless asked.`,
		Example: `  plumelog query apps
  plumelog query apps --source es
  plumelog query apps --app-like iipmes
  plumelog query apps --app-like zhiyi --enrich
  plumelog query apps --sample zhiyi-iipmes`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runApps(cmd, f)
		},
	}
	cmd.Flags().StringVar(&f.source, "source", "cache", "cache (getAppNames) or es (queryAppNames)")
	cmd.Flags().StringVar(&f.appLike, "app-like", "", "substring filter on app names (case-insensitive)")
	cmd.Flags().StringVar(&f.sample, "sample", "", "preview recent logs for this appName")
	cmd.Flags().BoolVar(&f.enrich, "enrich", false, "opt-in: last-log-time + sample className per app (N+1 queries, capped)")
	cmd.Flags().BoolVar(&f.enrich, "stats", false, "alias of --enrich")
	mark(cmd, risk.Read, "List application names")
	return cmd
}

func (a *App) runApps(cmd *cobra.Command, f appsFlags) error {
	req, err := appsRequest(f.source)
	if err != nil {
		return a.fail(cmd, risk.Read, err)
	}
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
	if code, ok := client.ResultCode(resp.Body); ok && code >= 400 {
		return a.finishUpstream(cmd, resp)
	}
	if resp.Status >= 400 {
		return a.finishUpstream(cmd, resp)
	}
	if len(strings.TrimSpace(string(resp.Body))) == 0 {
		return a.finishUpstream(cmd, resp)
	}

	names, shaped, err := parseAppNames(resp.Body)
	if err != nil {
		return a.fail(cmd, risk.Read, err)
	}
	like := strings.TrimSpace(f.appLike)
	if like != "" {
		names = filterAppLike(names, like)
		shaped = filterShapedAppLike(shaped, like)
	}

	out := map[string]any{
		"apps":  names,
		"count": len(names),
	}
	if like != "" {
		out["app_like"] = like
	}
	if shaped != nil {
		out["raw_shape"] = shaped["raw_shape"]
		if buckets, ok := shaped["buckets"]; ok {
			out["buckets"] = buckets
		}
	}

	if strings.TrimSpace(f.sample) != "" {
		sample, serr := a.fetchAppSample(cmd, strings.TrimSpace(f.sample), 3)
		if serr != nil {
			out["sample_error"] = serr.Error()
		} else {
			out["sample"] = sample
		}
	}

	if f.enrich {
		const capN = 20
		enrichNames := names
		truncated := false
		if len(enrichNames) > capN {
			enrichNames = enrichNames[:capN]
			truncated = true
		}
		stats := make([]any, 0, len(enrichNames))
		for _, name := range enrichNames {
			st, eerr := a.fetchAppEnrich(cmd, name)
			if eerr != nil {
				stats = append(stats, map[string]any{"app": name, "error": eerr.Error()})
				continue
			}
			stats = append(stats, st)
		}
		out["enrich"] = stats
		if truncated {
			out["enrich_truncated"] = true
			out["enrich_cap"] = capN
		}
	}

	env := output.Envelope{
		OK:     true,
		URL:    client.RedactURL(resp.FinalURL),
		Result: output.MustResult(out),
	}
	if like != "" && len(names) == 0 {
		env.Hint = "no apps matched --app-like; try a shorter substring or --source es"
	}
	return a.write(cmd, env)
}

func parseAppNames(body []byte) (names []string, shaped map[string]any, err error) {
	body = []byte(strings.TrimSpace(string(body)))
	var arr []any
	if json.Unmarshal(body, &arr) == nil {
		for _, v := range arr {
			switch t := v.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					names = append(names, s)
				}
			default:
				b, _ := json.Marshal(t)
				if s := strings.TrimSpace(string(b)); s != "" && s != "null" {
					names = append(names, s)
				}
			}
		}
		return names, map[string]any{"raw_shape": "array"}, nil
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, nil, fmt.Errorf("apps response is not JSON: %w", err)
	}
	// ES agg: aggregations.dataCount.buckets[].key
	if aggs, ok := root["aggregations"].(map[string]any); ok {
		if dc, ok := aggs["dataCount"].(map[string]any); ok {
			if buckets, ok := dc["buckets"].([]any); ok {
				var outBuckets []any
				for _, b := range buckets {
					m, ok := b.(map[string]any)
					if !ok {
						continue
					}
					key := fmt.Sprint(m["key"])
					if key == "" || key == "<nil>" {
						continue
					}
					names = append(names, key)
					outBuckets = append(outBuckets, map[string]any{
						"key":       key,
						"doc_count": m["doc_count"],
					})
				}
				return names, map[string]any{"raw_shape": "es_agg", "buckets": outBuckets}, nil
			}
		}
	}
	// common wrappers: {apps:[...]} / {data:[...]} / {result:[...]}
	for _, k := range []string{"apps", "data", "result", "list"} {
		if v, ok := root[k]; ok {
			raw, _ := json.Marshal(v)
			n, sh, e := parseAppNames(raw)
			if e == nil {
				if sh == nil {
					sh = map[string]any{}
				}
				sh["raw_shape"] = "object." + k
				return n, sh, nil
			}
		}
	}
	return nil, map[string]any{"raw_shape": "object"}, fmt.Errorf("unrecognized apps payload; use --source es or inspect raw with query dsl")
}

func filterAppLike(names []string, like string) []string {
	like = strings.ToLower(strings.TrimSpace(like))
	if like == "" {
		return names
	}
	var out []string
	for _, n := range names {
		if strings.Contains(strings.ToLower(n), like) {
			out = append(out, n)
		}
	}
	return out
}

func filterShapedAppLike(shaped map[string]any, like string) map[string]any {
	if shaped == nil {
		return nil
	}
	like = strings.ToLower(strings.TrimSpace(like))
	buckets, ok := shaped["buckets"].([]any)
	if !ok || like == "" {
		return shaped
	}
	var out []any
	for _, b := range buckets {
		m, ok := b.(map[string]any)
		if !ok {
			continue
		}
		key := strings.ToLower(fmt.Sprint(m["key"]))
		if strings.Contains(key, like) {
			out = append(out, b)
		}
	}
	shaped["buckets"] = out
	return shaped
}

func (a *App) fetchAppSample(cmd *cobra.Command, app string, size int) (any, error) {
	body, err := dsl.Build(dsl.Search{
		App:       app,
		FromMS:    time.Now().Add(-24 * time.Hour).UnixMilli(),
		ToMS:      time.Now().UnixMilli(),
		Highlight: false,
	})
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("size", strconv.Itoa(size))
	q.Set("from", "0")
	fromMS := time.Now().Add(-24 * time.Hour).UnixMilli()
	toMS := time.Now().UnixMilli()
	q.Set("clientStartDate", strconv.FormatInt(fromMS, 10))
	q.Set("clientEndDate", strconv.FormatInt(toMS, 10))
	req := client.Request{Method: "POST", Path: "/clientQuery", Query: q, Body: body}
	resp, err := a.do(cmd, req)
	if err != nil {
		return nil, err
	}
	if resp.Status >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.Status)
	}
	return summarizeSampleHits(resp.Body, size), nil
}

func (a *App) fetchAppEnrich(cmd *cobra.Command, app string) (map[string]any, error) {
	body, err := dsl.Build(dsl.Search{
		App:       app,
		FromMS:    time.Now().Add(-24 * time.Hour).UnixMilli(),
		ToMS:      time.Now().UnixMilli(),
		Highlight: false,
	})
	if err != nil {
		return nil, err
	}
	fromMS := time.Now().Add(-24 * time.Hour).UnixMilli()
	toMS := time.Now().UnixMilli()
	q := url.Values{}
	q.Set("size", "5")
	q.Set("from", "0")
	q.Set("clientStartDate", strconv.FormatInt(fromMS, 10))
	q.Set("clientEndDate", strconv.FormatInt(toMS, 10))
	req := client.Request{Method: "POST", Path: "/clientQuery", Query: q, Body: body}
	resp, err := a.do(cmd, req)
	if err != nil {
		return nil, err
	}
	if resp.Status >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.Status)
	}
	st := map[string]any{"app": app, "window": "24h"}
	total, ok := esHitsTotal(resp.Body)
	if ok {
		st["recent_count"] = total
	}
	last, classes := sampleMetaFromHits(resp.Body)
	if last != "" {
		st["last_log_time"] = last
	}
	if len(classes) > 0 {
		st["className_top"] = classes
	}
	return st, nil
}

func summarizeSampleHits(body []byte, limit int) map[string]any {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return map[string]any{"raw": output.RawResult(body)}
	}
	hitsObj, _ := root["hits"].(map[string]any)
	var hits []any
	if hitsObj != nil {
		hits, _ = hitsObj["hits"].([]any)
	}
	total, _ := esHitsTotal(body)
	outHits := make([]any, 0, limit)
	for i, h := range hits {
		if i >= limit {
			break
		}
		m, ok := h.(map[string]any)
		if !ok {
			continue
		}
		src, _ := m["_source"].(map[string]any)
		if src == nil {
			src = m
		}
		item := map[string]any{}
		for _, k := range []string{"dtTime", "className", "appName", "env", "logLevel", "content", "traceId"} {
			if v, ok := src[k]; ok {
				item[k] = v
			}
		}
		outHits = append(outHits, item)
	}
	return map[string]any{"total": total, "hits": outHits}
}

func sampleMetaFromHits(body []byte) (last string, classes []string) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return "", nil
	}
	hitsObj, _ := root["hits"].(map[string]any)
	if hitsObj == nil {
		return "", nil
	}
	hits, _ := hitsObj["hits"].([]any)
	counts := map[string]int{}
	for _, h := range hits {
		m, ok := h.(map[string]any)
		if !ok {
			continue
		}
		src, _ := m["_source"].(map[string]any)
		if src == nil {
			continue
		}
		if last == "" {
			switch v := src["dtTime"].(type) {
			case float64:
				last = strconv.FormatInt(int64(v), 10)
			case string:
				last = v
			default:
				if v != nil {
					last = fmt.Sprint(v)
				}
			}
		}
		if cn, ok := src["className"].(string); ok && cn != "" {
			counts[cn]++
		}
	}
	type kv struct {
		k string
		n int
	}
	var list []kv
	for k, n := range counts {
		list = append(list, kv{k, n})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		return list[i].k < list[j].k
	})
	for i, e := range list {
		if i >= 3 {
			break
		}
		classes = append(classes, e.k)
	}
	return last, classes
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
