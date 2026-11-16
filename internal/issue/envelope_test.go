package issue

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestBusinessEnvelope 锁定真实服务契约，避免旧版平铺夹具掩盖线上 400。
func TestBusinessEnvelope(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "error"}[failed], func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				meta, mok := body["metadata"].(map[string]any)
				data, dok := body["data"].(map[string]any)
				if !mok || !dok || len(body) != 2 {
					t.Errorf("missing envelope: %v", body)
				} else {
					page, _ := meta["page"].(map[string]any)
					if page["limit"] != float64(10) || data["page"] != nil {
						t.Errorf("wrong paging: %v", body)
					}
				}
				w.Header().Set("X-Request-Id", "trace-1")
				if failed {
					w.WriteHeader(400)
					_, _ = io.WriteString(w, `{"metadata":{"extra":{},"error":{"code":"VALIDATION_FAILED","message":"参数无效","details":{"field":"filter"}}},"data":null}`)
					return
				}
				_, _ = io.WriteString(w, `{"metadata":{"extra":{},"page":{"total":23,"nextOffset":10}},"data":{"items":[]}}`)
			}))
			defer srv.Close()
			cfg := Config{URL: srv.URL, Token: testToken}
			cli, err := NewClient(cfg, srv.Client())
			if err != nil {
				t.Fatal(err)
			}
			body := map[string]any{"page": map[string]any{"limit": 10}}
			out, err := cli.Do(context.Background(), "POST", "work-items/search", nil, body)
			if failed {
				var ae *APIError
				if !errors.As(err, &ae) || ae.Code != "VALIDATION_FAILED" || ae.RequestID != "trace-1" || ae.Details == nil {
					t.Fatalf("%+v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data := out.(map[string]any)
			if data["total"] != float64(23) || data["nextOffset"] != float64(10) || data["items"] == nil {
				t.Fatal(data)
			}
		})
	}
}
