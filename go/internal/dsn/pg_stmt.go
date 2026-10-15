package dsn

import (
	"regexp"
	"strings"
)

// PgCatalog 不是 SQL。Gate 和驱动必须认同一套 TABLES / COLUMNS / DDL 形状。
type PgCatalog struct {
	Head    string
	Like    string
	Rel     string
	HasLike bool
}

var relNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

func pgCatalogHead(stmt string) string {
	args := splitArgs(stmt)
	if len(args) == 0 {
		return ""
	}
	head := strings.ToUpper(args[0])
	switch head {
	case "TABLES", "COLUMNS", "DDL", "DESC", "DESCRIBE":
		return head
	default:
		return ""
	}
}

func parsePgCatalog(stmt string) (PgCatalog, error) {
	args := splitArgs(stmt)
	if len(args) == 0 || args[0] == "" {
		return PgCatalog{}, exitf(2, "empty statement")
	}
	head := strings.ToUpper(args[0])
	if head == "TABLES" {
		return parseTables(args)
	}
	switch head {
	case "COLUMNS", "DDL", "DESC", "DESCRIBE":
		rel, err := parseRel(args)
		if err != nil {
			return PgCatalog{}, err
		}
		if head == "DDL" {
			return PgCatalog{Head: "ddl", Rel: rel}, nil
		}
		return PgCatalog{Head: "columns", Rel: rel}, nil
	default:
		return PgCatalog{}, exitf(2, "blocked head: %s", args[0])
	}
}

func parseTables(args []string) (PgCatalog, error) {
	if len(args) == 1 {
		return PgCatalog{Head: "tables"}, nil
	}
	if len(args) == 3 && strings.ToUpper(args[1]) == "LIKE" {
		if args[2] == "" {
			return PgCatalog{}, exitf(2, "tables LIKE requires a pattern")
		}
		return PgCatalog{Head: "tables", Like: args[2], HasLike: true}, nil
	}
	return PgCatalog{}, exitf(2, "tables query must be TABLES or TABLES LIKE <pattern>")
}

func parseRel(args []string) (string, error) {
	cmd := "COLUMNS"
	if len(args) > 0 && args[0] != "" {
		cmd = strings.ToUpper(args[0])
	}
	if len(args) != 2 {
		return "", exitf(2, "%s query must be %s <table>", cmd, cmd)
	}
	rel := args[1]
	if rel == "" {
		return "", exitf(2, "%s requires a table", cmd)
	}
	if !relNameRe.MatchString(rel) {
		return "", exitf(2, "invalid table name")
	}
	return rel, nil
}

func pgCatalogComplete(rest []string) []string {
	heads := []string{"tables", "columns", "ddl", "desc"}
	if len(rest) == 0 {
		return heads
	}
	head := strings.ToLower(rest[0])
	if head == "tables" {
		if len(rest) == 1 {
			return []string{"like"}
		}
		return []string{}
	}
	switch head {
	case "columns", "ddl", "desc", "describe":
		return []string{}
	default:
		return heads
	}
}
