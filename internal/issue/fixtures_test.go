package issue

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// fixtureBody 校验传输 envelope 后提供业务视图，让领域断言聚焦权限、并发与确认边界。
func fixtureBody(t *testing.T, r *http.Request) io.Reader {
	t.Helper()
	var env struct {
		Metadata map[string]any `json:"metadata"`
		Data     map[string]any `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Metadata == nil || env.Data == nil {
		t.Fatal("request missing metadata/data")
	}
	if page, ok := env.Metadata["page"]; ok {
		env.Data["page"] = page
	}
	raw, err := json.Marshal(env.Data)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(raw)
}

type fixtureWriter struct{ http.ResponseWriter }

// Write 将业务夹具编码为服务端 envelope，错误追踪号按真实协议放入响应头。
func (w fixtureWriter) Write(raw []byte) (int, error) {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return w.ResponseWriter.Write(raw)
	}
	meta := map[string]any{"extra": map[string]string{}}
	if data["code"] != nil {
		if id, ok := data["requestId"].(string); ok {
			w.Header().Set("X-Request-Id", id)
			delete(data, "requestId")
		}
		meta["error"] = data
		data = nil
	} else {
		page := map[string]any{}
		for _, key := range []string{"total", "nextOffset", "nextCursor"} {
			if v, ok := data[key]; ok {
				page[key] = v
				delete(data, key)
			}
		}
		if len(page) > 0 {
			meta["page"] = page
		}
	}
	env := map[string]any{"metadata": meta, "data": data}
	buf, err := json.Marshal(env)
	if err != nil {
		return 0, err
	}
	_, err = w.ResponseWriter.Write(buf)
	return len(raw), err
}
