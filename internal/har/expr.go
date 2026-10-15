package har

import (
	"fmt"
	"math"
	"unicode"
	"unicode/utf8"
)

type tokKind int

const (
	tokIdent tokKind = iota
	tokStr
	tokNum
	tokBool
	tokNull
	tokOp
	tokParen
)

type token struct {
	kind tokKind
	text string
	op   string
	str  string
	num  float64
	b    bool
}

// Eval 计算 where 表达式。
// 不能用 eval，字段缺失时要得到 false，而不是把整次扫描打崩。
func Eval(expr string, ctx jVal) (bool, error) {
	toks, err := tokenize(expr)
	if err != nil {
		return false, err
	}
	p := parser{toks: toks, ctx: ctx}
	val, err := p.parse()
	if err != nil {
		return false, err
	}
	return truthy(val), nil
}

func tokenize(expr string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(expr) {
		r, size := utf8.DecodeRuneInString(expr[i:])
		if unicode.IsSpace(r) {
			i += size
			continue
		}
		if op, n, paren := matchOp(expr, i); n > 0 {
			if paren {
				toks = append(toks, token{kind: tokParen, text: op, op: op})
			} else {
				toks = append(toks, token{kind: tokOp, text: op, op: op})
			}
			i += n
			continue
		}
		if r >= '0' && r <= '9' {
			raw, next := readWhile(expr, i, isNumChar)
			n := jsNumber(raw)
			toks = append(toks, token{kind: tokNum, text: numText(n), num: n})
			i = next
			continue
		}
		if isIdentStart(r) {
			raw, next := readWhile(expr, i, isIdentChar)
			switch raw {
			case "and", "or", "not":
				toks = append(toks, token{kind: tokOp, text: raw, op: raw})
			case "true", "false":
				toks = append(toks, token{kind: tokBool, text: raw, b: raw == "true"})
			case "null":
				toks = append(toks, token{kind: tokNull, text: "null"})
			default:
				toks = append(toks, token{kind: tokIdent, text: raw, str: raw})
			}
			i = next
			continue
		}
		if r == '\'' || r == '"' {
			val, next, err := readString(expr, i)
			if err != nil {
				return nil, err
			}
			toks = append(toks, token{kind: tokStr, text: val, str: val})
			i = next
			continue
		}
		return nil, fmt.Errorf("Unsupported token near: %s", near(expr, i))
	}
	return toks, nil
}

func near(expr string, i int) string {
	end := i
	count := 0
	for end < len(expr) && count < 10 {
		_, size := utf8.DecodeRuneInString(expr[end:])
		end += size
		count++
	}
	return expr[i:end]
}

func matchOp(expr string, i int) (string, int, bool) {
	ops := []string{"==", "!=", ">=", "<=", "&&", "||", ">", "<", "(", ")"}
	for _, op := range ops {
		if i+len(op) <= len(expr) && expr[i:i+len(op)] == op {
			return op, len(op), op == "(" || op == ")"
		}
	}
	return "", 0, false
}

func readWhile(expr string, i int, ok func(byte) bool) (string, int) {
	j := i
	for j < len(expr) && ok(expr[j]) {
		j++
	}
	return expr[i:j], j
}

func isNumChar(b byte) bool {
	return b >= '0' && b <= '9' || b == '.'
}

func isIdentStart(r rune) bool {
	return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '_'
}

func isIdentChar(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '_' || b == '.'
}

func readString(expr string, i int) (string, int, error) {
	quote := expr[i]
	j := i + 1
	for j < len(expr) && expr[j] != quote {
		j++
	}
	if j >= len(expr) {
		return "", 0, fmt.Errorf("Unterminated string literal")
	}
	return expr[i+1 : j], j + 1, nil
}

type parser struct {
	toks []token
	ctx  jVal
	i    int
}

func (p *parser) parse() (jVal, error) {
	val, err := p.parseOr()
	if err != nil {
		return undef(), err
	}
	if p.i != len(p.toks) {
		return undef(), fmt.Errorf("Unexpected token: %s", p.toks[p.i].text)
	}
	return val, nil
}

func (p *parser) parseOr() (jVal, error) {
	left, err := p.parseAnd()
	if err != nil {
		return undef(), err
	}
	for p.matchOp("or", "||") {
		right, err := p.parseAnd()
		if err != nil {
			return undef(), err
		}
		left = jBool(truthy(left) || truthy(right))
	}
	return left, nil
}

func (p *parser) parseAnd() (jVal, error) {
	left, err := p.parseNot()
	if err != nil {
		return undef(), err
	}
	for p.matchOp("and", "&&") {
		right, err := p.parseNot()
		if err != nil {
			return undef(), err
		}
		left = jBool(truthy(left) && truthy(right))
	}
	return left, nil
}

func (p *parser) parseNot() (jVal, error) {
	if p.matchOp("not", "!") {
		val, err := p.parseNot()
		if err != nil {
			return undef(), err
		}
		return jBool(!truthy(val)), nil
	}
	return p.parseCmp()
}

func (p *parser) parseCmp() (jVal, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return undef(), err
	}
	op := p.peek()
	if op == nil || op.kind != tokOp || !isCmp(op.op) {
		return left, nil
	}
	p.i++
	right, err := p.parsePrimary()
	if err != nil {
		return undef(), err
	}
	switch op.op {
	case "==":
		return jBool(jsEqual(left, right)), nil
	case "!=":
		return jBool(!jsEqual(left, right)), nil
	case ">":
		return jBool(cmp(left, right, func(a, b float64) bool { return a > b })), nil
	case ">=":
		return jBool(cmp(left, right, func(a, b float64) bool { return a >= b })), nil
	case "<":
		return jBool(cmp(left, right, func(a, b float64) bool { return a < b })), nil
	case "<=":
		return jBool(cmp(left, right, func(a, b float64) bool { return a <= b })), nil
	default:
		return undef(), fmt.Errorf("Unsupported operator: %s", op.op)
	}
}

func cmp(left, right jVal, fn func(float64, float64) bool) bool {
	a := jsNum(left)
	b := jsNum(right)
	if math.IsNaN(a) || math.IsNaN(b) {
		return false
	}
	return fn(a, b)
}

func isCmp(op string) bool {
	switch op {
	case "==", "!=", ">", ">=", "<", "<=":
		return true
	default:
		return false
	}
}

func (p *parser) parsePrimary() (jVal, error) {
	tok := p.peek()
	if tok == nil {
		return undef(), fmt.Errorf("Unexpected end of expression")
	}
	if tok.kind == tokParen && tok.op == "(" {
		p.i++
		val, err := p.parseOr()
		if err != nil {
			return undef(), err
		}
		if err := p.expect(")"); err != nil {
			return undef(), err
		}
		return val, nil
	}
	p.i++
	switch tok.kind {
	case tokIdent:
		return p.ctx.path(tok.str), nil
	case tokNum:
		return jNum(tok.num), nil
	case tokStr:
		return jStr(tok.str), nil
	case tokBool:
		return jBool(tok.b), nil
	case tokNull:
		return jNull(), nil
	default:
		return undef(), fmt.Errorf("Unexpected token: %s", tok.text)
	}
}

func (p *parser) matchOp(ops ...string) bool {
	tok := p.peek()
	if tok == nil || tok.kind != tokOp {
		return false
	}
	for _, op := range ops {
		if tok.op == op {
			p.i++
			return true
		}
	}
	return false
}

func (p *parser) expect(want string) error {
	tok := p.peek()
	if tok == nil || tok.kind != tokParen || tok.op != want {
		return fmt.Errorf("Expected %s", want)
	}
	p.i++
	return nil
}

func (p *parser) peek() *token {
	if p.i >= len(p.toks) {
		return nil
	}
	return &p.toks[p.i]
}
