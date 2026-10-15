package dsn

import (
	"context"
	"database/sql"
	"net"
	"net/url"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func queryMySQL(ctx context.Context, raw, stmt string, connect, exec time.Duration) (QueryOut, error) {
	cfg, err := mysqlConfig(raw, connect, exec)
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	dialCtx, cancel := context.WithTimeout(ctx, connect)
	err = db.PingContext(dialCtx)
	cancel()
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	qctx, qcancel := context.WithTimeout(ctx, exec)
	defer qcancel()
	if mysqlReturnsRows(stmt) {
		return mysqlQuery(qctx, db, stmt)
	}
	return mysqlExec(qctx, db, stmt)
}

func mysqlConfig(raw string, connect, exec time.Duration) (*mysqldriver.Config, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	cfg := mysqldriver.NewConfig()
	cfg.User = parsed.User.Username()
	pass, _ := parsed.User.Password()
	cfg.Passwd = pass
	cfg.Net = "tcp"
	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		port = "3306"
	}
	cfg.Addr = net.JoinHostPort(host, port)
	cfg.DBName = trimSlash(parsed.Path)
	cfg.Timeout = connect
	cfg.ReadTimeout = exec
	cfg.WriteTimeout = exec
	cfg.ParseTime = true
	cfg.Params = map[string]string{}
	for key, vals := range parsed.Query() {
		if len(vals) > 0 {
			cfg.Params[key] = vals[0]
		}
	}
	return cfg, nil
}

func trimSlash(path string) string {
	for len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	if path == "" {
		return ""
	}
	dec, err := url.PathUnescape(path)
	if err != nil {
		return path
	}
	return dec
}

func mysqlReturnsRows(stmt string) bool {
	words := sqlWords(stmt, "mysql")
	if len(words) == 0 {
		return true
	}
	switch words[0] {
	case "SELECT", "WITH", "SHOW", "EXPLAIN", "DESC", "DESCRIBE":
		return true
	default:
		return false
	}
}

func mysqlQuery(ctx context.Context, db *sql.DB, stmt string) (QueryOut, error) {
	rows, err := db.QueryContext(ctx, stmt)
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	if len(cols) == 0 {
		if rows.NextResultSet() {
			return QueryOut{}, exitf(3, "multiple result sets")
		}
		return QueryOut{
			Columns: []string{"affectedRows", "insertId"},
			Rows:    []map[string]any{{"affectedRows": int64(0), "insertId": int64(0)}},
		}, nil
	}
	var out []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
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
	if rows.NextResultSet() {
		return QueryOut{}, exitf(3, "multiple result sets")
	}
	if out == nil {
		out = []map[string]any{}
	}
	return QueryOut{Columns: cols, Rows: out}, nil
}

func mysqlExec(ctx context.Context, db *sql.DB, stmt string) (QueryOut, error) {
	res, err := db.ExecContext(ctx, stmt)
	if err != nil {
		return QueryOut{}, exitf(3, "%s", err.Error())
	}
	affected, _ := res.RowsAffected()
	insertID, _ := res.LastInsertId()
	return QueryOut{
		Columns: []string{"affectedRows", "insertId"},
		Rows:    []map[string]any{{"affectedRows": affected, "insertId": insertID}},
	}, nil
}

func normSQL(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	case nil:
		return nil
	default:
		return t
	}
}
