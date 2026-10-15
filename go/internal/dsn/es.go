package dsn

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

func queryES(ctx context.Context, raw, stmt string, connect, exec time.Duration) (QueryOut, error) {
	parsed, err := parseRestQuery(stmt)
	if err != nil {
		return QueryOut{}, err
	}
	node, user, pass, err := esNode(raw)
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout: connect,
		}).DialContext,
		TLSHandshakeTimeout: connect,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
	}
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses:    []string{node},
		Username:     user,
		Password:     pass,
		Transport:    transport,
		DisableRetry: true,
		MaxRetries:   0,
	})
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	qctx, cancel := context.WithTimeout(ctx, exec)
	defer cancel()
	var body io.Reader
	if len(parsed.Body) > 0 {
		body = bytes.NewReader(parsed.Body)
	}
	req, err := http.NewRequestWithContext(qctx, parsed.Method, parsed.Path, body)
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	if len(parsed.Body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := client.Perform(req)
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	defer res.Body.Close()
	buf, err := io.ReadAll(res.Body)
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	if res.StatusCode >= 400 {
		msg := string(buf)
		if msg == "" {
			msg = res.Status
		}
		return QueryOut{}, exitf(3, "%s", msg)
	}
	return esOut(buf), nil
}

func esNode(raw string) (string, string, string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", "", err
	}
	user := parsed.User.Username()
	pass, _ := parsed.User.Password()
	parsed.User = nil
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), user, pass, nil
}

func esOut(buf []byte) QueryOut {
	if len(bytes.TrimSpace(buf)) == 0 {
		return docsOut([]any{map[string]any{}})
	}
	var decoded any
	if err := json.Unmarshal(buf, &decoded); err != nil {
		return docsOut([]any{map[string]any{"value": string(buf)}})
	}
	if obj, ok := decoded.(map[string]any); ok {
		if hits, ok := obj["hits"].(map[string]any); ok {
			if arr, ok := hits["hits"].([]any); ok {
				return docsOut(arr)
			}
		}
	}
	return docsOut([]any{decoded})
}

func docsOut(docs []any) QueryOut {
	rows := make([]map[string]any, len(docs))
	for i, doc := range docs {
		rows[i] = asRow(doc)
	}
	cols := []string{}
	if len(rows) > 0 {
		cols = rowKeys(nil, rows[0])
	}
	return QueryOut{Columns: cols, Rows: rows}
}

func asRow(value any) map[string]any {
	if obj, ok := value.(map[string]any); ok {
		return obj
	}
	return map[string]any{"value": value}
}
