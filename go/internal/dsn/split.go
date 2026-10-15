package dsn

import "unicode"

// splitArgs 按空白和引号切开语句。Redis 和 kafka 的头永远是第一个 token，引号里的空格不能切开。
func splitArgs(stmt string) []string {
	var args []string
	i := 0
	n := len(stmt)
	for i < n {
		for i < n && unicode.IsSpace(rune(stmt[i])) {
			i++
		}
		if i >= n {
			break
		}
		ch := stmt[i]
		if ch == '\'' || ch == '"' {
			start := i + 1
			i++
			for i < n && stmt[i] != ch {
				if stmt[i] == '\\' {
					i += 2
					continue
				}
				i++
			}
			end := i
			if end > n {
				end = n
			}
			args = append(args, stmt[start:end])
			if i < n {
				i++
			}
			continue
		}
		start := i
		for i < n && !unicode.IsSpace(rune(stmt[i])) {
			i++
		}
		args = append(args, stmt[start:i])
	}
	return args
}
