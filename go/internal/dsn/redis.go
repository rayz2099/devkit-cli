package dsn

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func queryRedis(ctx context.Context, raw, stmt string, connect, exec time.Duration) (QueryOut, error) {
	args := splitArgs(stmt)
	if len(args) == 0 || args[0] == "" {
		return QueryOut{}, exitf(2, "empty statement")
	}
	opt, err := goredis.ParseURL(raw)
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	// -1 关掉默认重试。探测失败必须停，不能把超时拖过 deadline。
	opt.MaxRetries = -1
	opt.MinRetryBackoff = -1
	opt.MaxRetryBackoff = -1
	opt.DialTimeout = connect
	opt.ReadTimeout = exec
	opt.WriteTimeout = exec
	opt.ContextTimeoutEnabled = true
	client := goredis.NewClient(opt)
	defer client.Close()
	qctx, cancel := context.WithTimeout(ctx, connect+exec)
	defer cancel()
	call := make([]any, len(args))
	for i, arg := range args {
		call[i] = arg
	}
	val, err := client.Do(qctx, call...).Result()
	if errors.Is(err, goredis.Nil) {
		return valueOut(nil), nil
	}
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	return valueOut(normRedis(val)), nil
}

func normRedis(v any) any {
	switch t := v.(type) {
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = normRedis(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			out[k] = normRedis(item)
		}
		return out
	default:
		return v
	}
}

func valueOut(value any) QueryOut {
	if arr, ok := value.([]any); ok {
		rows := make([]map[string]any, len(arr))
		for i, item := range arr {
			rows[i] = map[string]any{"value": item}
		}
		return QueryOut{Columns: []string{"value"}, Rows: rows}
	}
	return QueryOut{Columns: []string{"value"}, Rows: []map[string]any{{"value": value}}}
}
