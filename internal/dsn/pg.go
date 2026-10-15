package dsn

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func queryPostgres(ctx context.Context, raw, stmt string, connect, exec time.Duration) (QueryOut, error) {
	cfg, err := pgx.ParseConfig(raw)
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	cfg.ConnectTimeout = connect
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	cfg.RuntimeParams["statement_timeout"] = strconv.FormatInt(exec.Milliseconds(), 10)
	dialCtx, cancel := context.WithTimeout(ctx, connect)
	conn, err := pgx.ConnectConfig(dialCtx, cfg)
	cancel()
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	defer conn.Close(context.Background())
	qctx, qcancel := context.WithTimeout(ctx, exec)
	defer qcancel()
	if pgCatalogHead(stmt) != "" {
		spec, err := parsePgCatalog(stmt)
		if err != nil {
			return QueryOut{}, err
		}
		return runPgCatalog(qctx, conn, spec)
	}
	rows, err := conn.Query(qctx, stmt)
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	return pgRows(rows)
}

func pgRows(rows pgx.Rows) (QueryOut, error) {
	defer rows.Close()
	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = string(fd.Name)
	}
	var out []map[string]any
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return QueryOut{}, exitf(3, "%s", err.Error())
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = normSQL(vals[i])
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	if len(cols) == 0 {
		tag := rows.CommandTag()
		word := tag.String()
		if i := strings.IndexByte(word, ' '); i > 0 {
			word = word[:i]
		}
		return QueryOut{
			Columns: []string{"command", "rowCount"},
			Rows:    []map[string]any{{"command": word, "rowCount": tag.RowsAffected()}},
		}, nil
	}
	if out == nil {
		out = []map[string]any{}
	}
	return QueryOut{Columns: cols, Rows: out}, nil
}
