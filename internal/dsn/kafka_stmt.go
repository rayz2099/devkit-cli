package dsn

import "strconv"

const (
	peekDefault = 50
	kafkaMax    = 500
)

// KafkaStmt 是 Gate 和驱动共用的语句。头必须小写，补全出来的 token 才能和拦截对上。
type KafkaStmt struct {
	Head      string
	Topic     string
	N         int
	Partition *int
}

func parseKafkaStmt(stmt string) (KafkaStmt, error) {
	args := splitArgs(stmt)
	if len(args) == 0 || args[0] == "" {
		return KafkaStmt{}, exitf(2, "empty statement")
	}
	switch args[0] {
	case "topics":
		if len(args) > 1 {
			return KafkaStmt{}, exitf(2, "topics takes no arguments")
		}
		return KafkaStmt{Head: "topics"}, nil
	case "peek":
		return parsePeek(args[1:])
	case "listen":
		return parseListen(args[1:])
	default:
		return KafkaStmt{}, exitf(2, "blocked head: %s", args[0])
	}
}

func isKafkaListen(stmt string) bool {
	args := splitArgs(stmt)
	return len(args) > 0 && args[0] == "listen"
}

func isKafkaTopics(stmt string) bool {
	args := splitArgs(stmt)
	return len(args) > 0 && args[0] == "topics"
}

func kafkaComplete(rest, topics []string) []string {
	if len(rest) == 0 {
		return []string{"topics", "peek", "listen"}
	}
	if rest[0] == "topics" {
		return []string{}
	}
	if rest[0] == "peek" || rest[0] == "listen" {
		if len(rest) == 1 {
			return topics
		}
		return []string{}
	}
	return []string{"topics", "peek", "listen"}
}

func parsePeek(args []string) (KafkaStmt, error) {
	if len(args) == 0 || args[0] == "" {
		return KafkaStmt{}, exitf(2, "peek requires a topic")
	}
	out := KafkaStmt{Head: "peek", Topic: args[0], N: peekDefault}
	i := 1
	if i < len(args) && args[i] != "partition" {
		n, err := readCount(args[i])
		if err != nil {
			return KafkaStmt{}, err
		}
		out.N = n
		i++
	}
	if i < len(args) && args[i] == "partition" {
		part, err := readPart(argAt(args, i+1))
		if err != nil {
			return KafkaStmt{}, err
		}
		out.Partition = &part
		i += 2
	}
	if i != len(args) {
		return KafkaStmt{}, exitf(2, "invalid peek statement")
	}
	return out, nil
}

func parseListen(args []string) (KafkaStmt, error) {
	if len(args) == 0 || args[0] == "" {
		return KafkaStmt{}, exitf(2, "listen requires a topic")
	}
	out := KafkaStmt{Head: "listen", Topic: args[0]}
	i := 1
	if i < len(args) && args[i] == "partition" {
		part, err := readPart(argAt(args, i+1))
		if err != nil {
			return KafkaStmt{}, err
		}
		out.Partition = &part
		i += 2
	}
	if i != len(args) {
		return KafkaStmt{}, exitf(2, "invalid listen statement")
	}
	return out, nil
}

func argAt(args []string, i int) string {
	if i < 0 || i >= len(args) {
		return ""
	}
	return args[i]
}

func readCount(raw string) (int, error) {
	if raw == "" || raw[0] < '1' || raw[0] > '9' {
		return 0, exitf(2, "peek n must be 1..%d", kafkaMax)
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, exitf(2, "peek n must be 1..%d", kafkaMax)
		}
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n > kafkaMax {
		return 0, exitf(2, "peek n must be 1..%d", kafkaMax)
	}
	return n, nil
}

func readPart(raw string) (int, error) {
	if raw == "" {
		return 0, exitf(2, "partition id required")
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, exitf(2, "partition id required")
		}
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, exitf(2, "partition id required")
	}
	return n, nil
}
