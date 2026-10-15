package dsn

import (
	"context"
	"math"
	"time"
)

type timeouts struct {
	connect time.Duration
	exec    time.Duration
}

func ms(sec float64) time.Duration {
	return time.Duration(math.Round(sec*1000)) * time.Millisecond
}

// runDriver 走进程内驱动。官方客户端只留给 Console，这样出站前才能过 Gate。
func runDriver(ctx context.Context, kind, raw, stmt string, limit timeouts) (QueryOut, error) {
	var out QueryOut
	var err error
	switch kind {
	case "mysql", "doris":
		out, err = queryMySQL(ctx, raw, stmt, limit.connect, limit.exec)
	case "postgres":
		out, err = queryPostgres(ctx, raw, stmt, limit.connect, limit.exec)
	case "redis":
		out, err = queryRedis(ctx, raw, stmt, limit.connect, limit.exec)
	case "mongodb":
		out, err = queryMongo(ctx, raw, stmt, limit.connect, limit.exec)
	case "elasticsearch":
		out, err = queryES(ctx, raw, stmt, limit.connect, limit.exec)
	case "kafka":
		out, err = queryKafka(ctx, raw, stmt, limit.connect, limit.exec)
	default:
		return QueryOut{}, exitf(3, "unknown kind %s", kind)
	}
	if err != nil {
		if asExit(err) != nil {
			return QueryOut{}, err
		}
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	return out, nil
}
