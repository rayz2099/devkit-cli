package har

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// 过滤和 JSON 投影都按 JavaScript 的值来比。
// Go 的强类型相等会把缺字段和 "1" == 1 判错，HAR 表达式对不上。
type kind int

const (
	kindUndef kind = iota
	kindNull
	kindBool
	kindNum
	kindStr
	kindArr
	kindObj
)

type pair struct {
	key string
	val jVal
}

// ordMap 记住字段插入顺序。
// encoding/json 的 map 会按字母重排，投影结果就和 JSON.stringify 不一致。
type ordMap struct {
	keys []string
	vals map[string]jVal
}

func newMap() *ordMap {
	return &ordMap{vals: map[string]jVal{}}
}

func (m *ordMap) put(key string, val jVal) {
	if _, ok := m.vals[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.vals[key] = val
}

func (m *ordMap) get(key string) (jVal, bool) {
	val, ok := m.vals[key]
	return val, ok
}

type jVal struct {
	k kind
	b bool
	n float64
	s string
	a []jVal
	o *ordMap
}

func undef() jVal            { return jVal{k: kindUndef} }
func jNull() jVal            { return jVal{k: kindNull} }
func jBool(b bool) jVal      { return jVal{k: kindBool, b: b} }
func jNum(n float64) jVal    { return jVal{k: kindNum, n: n} }
func jStr(s string) jVal     { return jVal{k: kindStr, s: s} }
func jArr(items []jVal) jVal { return jVal{k: kindArr, a: items} }
func jObj(m *ordMap) jVal    { return jVal{k: kindObj, o: m} }

func (v jVal) child(seg string) jVal {
	switch v.k {
	case kindObj:
		if v.o == nil {
			return undef()
		}
		got, ok := v.o.get(seg)
		if !ok {
			return undef()
		}
		return got
	case kindArr:
		idx, err := strconv.Atoi(seg)
		if err != nil || strconv.Itoa(idx) != seg || idx < 0 || idx >= len(v.a) {
			return undef()
		}
		return v.a[idx]
	default:
		return undef()
	}
}

func (v jVal) path(p string) jVal {
	cur := v
	for _, seg := range strings.Split(p, ".") {
		cur = cur.child(seg)
		if cur.k == kindUndef {
			return cur
		}
	}
	return cur
}

// setPath 按 a.b.c 写入投影。
// 中间碰到数组或标量就换成对象，和原来的字段投影一样，后写的字段能盖住先写的整段。
func setPath(root *ordMap, path string, val jVal) {
	segs := strings.Split(path, ".")
	cur := root
	last := len(segs) - 1
	for i := 0; i < last; i++ {
		seg := segs[i]
		next, ok := cur.get(seg)
		if !ok || next.k != kindObj || next.o == nil {
			child := newMap()
			cur.put(seg, jObj(child))
			cur = child
			continue
		}
		cur = next.o
	}
	cur.put(segs[last], val)
}

func truthy(v jVal) bool {
	switch v.k {
	case kindUndef, kindNull:
		return false
	case kindBool:
		return v.b
	case kindNum:
		return v.n != 0 && !math.IsNaN(v.n)
	case kindStr:
		return v.s != ""
	default:
		return true
	}
}

func jsNum(v jVal) float64 {
	switch v.k {
	case kindUndef:
		return math.NaN()
	case kindNull:
		return 0
	case kindBool:
		if v.b {
			return 1
		}
		return 0
	case kindNum:
		return v.n
	case kindStr:
		return jsNumber(v.s)
	default:
		return jsNumber(primString(v))
	}
}

func primString(v jVal) string {
	switch v.k {
	case kindUndef:
		return "undefined"
	case kindNull:
		return "null"
	case kindBool:
		if v.b {
			return "true"
		}
		return "false"
	case kindNum:
		return numText(v.n)
	case kindStr:
		return v.s
	case kindArr:
		parts := make([]string, len(v.a))
		for i, el := range v.a {
			if el.k == kindUndef || el.k == kindNull {
				continue
			}
			parts[i] = primString(el)
		}
		return strings.Join(parts, ",")
	default:
		return "[object Object]"
	}
}

func numText(n float64) string {
	if math.IsNaN(n) {
		return "NaN"
	}
	if math.IsInf(n, 1) {
		return "Infinity"
	}
	if math.IsInf(n, -1) {
		return "-Infinity"
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}

// jsEqual 按 ECMAScript 抽象相等比较。
// 过滤表达式用的是 ==，不是 Go 的 ==，字符串和数字要能对上。
func jsEqual(a, b jVal) bool {
	if a.k == b.k {
		switch a.k {
		case kindUndef, kindNull:
			return true
		case kindNum:
			if math.IsNaN(a.n) || math.IsNaN(b.n) {
				return false
			}
			return a.n == b.n
		case kindStr:
			return a.s == b.s
		case kindBool:
			return a.b == b.b
		default:
			return false
		}
	}
	if a.k == kindNull && b.k == kindUndef || a.k == kindUndef && b.k == kindNull {
		return true
	}
	if a.k == kindNum && b.k == kindStr {
		return jsEqual(a, jNum(jsNumber(b.s)))
	}
	if a.k == kindStr && b.k == kindNum {
		return jsEqual(jNum(jsNumber(a.s)), b)
	}
	if a.k == kindBool {
		n := 0.0
		if a.b {
			n = 1
		}
		return jsEqual(jNum(n), b)
	}
	if b.k == kindBool {
		n := 0.0
		if b.b {
			n = 1
		}
		return jsEqual(a, jNum(n))
	}
	if objLike(a.k) && (b.k == kindStr || b.k == kindNum) {
		return jsEqual(jStr(primString(a)), b)
	}
	if objLike(b.k) && (a.k == kindStr || a.k == kindNum) {
		return jsEqual(a, jStr(primString(b)))
	}
	return false
}

func objLike(k kind) bool {
	return k == kindObj || k == kindArr
}

var decRe = regexp.MustCompile(`^(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?$`)

// jsNumber 对齐 JavaScript 的 Number(string)。
// --prefix-depth 用的就是这个转换，"2.0" 和 "0x10" 都要能过整数检查。
func jsNumber(raw string) float64 {
	s := strings.TrimFunc(raw, unicode.IsSpace)
	if s == "" {
		return 0
	}
	sign := 1.0
	if strings.HasPrefix(s, "+") {
		s = s[1:]
	} else if strings.HasPrefix(s, "-") {
		sign = -1
		s = s[1:]
	}
	if s == "" {
		return math.NaN()
	}
	if s == "Infinity" {
		return sign * math.Inf(1)
	}
	if len(s) >= 2 && s[0] == '0' {
		switch s[1] {
		case 'x', 'X', 'b', 'B', 'o', 'O':
			return sign * parseRadix(s)
		}
	}
	if !decRe.MatchString(s) {
		return math.NaN()
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}
	return sign * n
}

func parseRadix(s string) float64 {
	base := 16
	switch s[1] {
	case 'b', 'B':
		base = 2
	case 'o', 'O':
		base = 8
	}
	digits := s[2:]
	if digits == "" {
		return math.NaN()
	}
	n, err := strconv.ParseUint(digits, base, 64)
	if err != nil {
		return math.NaN()
	}
	return float64(n)
}

func parseJSONVal(text string) (jVal, bool) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	val, err := readJSON(dec)
	if err != nil {
		return undef(), false
	}
	if dec.More() {
		return undef(), false
	}
	return val, true
}

func readJSON(dec *json.Decoder) (jVal, error) {
	tok, err := dec.Token()
	if err != nil {
		return undef(), err
	}
	switch cur := tok.(type) {
	case json.Delim:
		if cur == '{' {
			return readObj(dec)
		}
		if cur == '[' {
			return readArr(dec)
		}
		return undef(), errBadJSON
	case nil:
		return jNull(), nil
	case bool:
		return jBool(cur), nil
	case json.Number:
		n, err := cur.Float64()
		if err != nil {
			return undef(), err
		}
		return jNum(n), nil
	case string:
		return jStr(cur), nil
	default:
		return undef(), errBadJSON
	}
}

func readObj(dec *json.Decoder) (jVal, error) {
	m := newMap()
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return undef(), err
		}
		key, ok := tok.(string)
		if !ok {
			return undef(), errBadJSON
		}
		val, err := readJSON(dec)
		if err != nil {
			return undef(), err
		}
		m.put(key, val)
	}
	end, err := dec.Token()
	if err != nil {
		return undef(), err
	}
	if end != json.Delim('}') {
		return undef(), errBadJSON
	}
	return jObj(m), nil
}

func readArr(dec *json.Decoder) (jVal, error) {
	var items []jVal
	for dec.More() {
		val, err := readJSON(dec)
		if err != nil {
			return undef(), err
		}
		items = append(items, val)
	}
	end, err := dec.Token()
	if err != nil {
		return undef(), err
	}
	if end != json.Delim(']') {
		return undef(), errBadJSON
	}
	if items == nil {
		items = []jVal{}
	}
	return jArr(items), nil
}

var errBadJSON = errors.New("invalid json")

func (v jVal) compact() string {
	return stringify(v, false)
}

func stringify(v jVal, pretty bool) string {
	var buf bytes.Buffer
	pad := ""
	if pretty {
		pad = "  "
	}
	writeJSON(&buf, v, pad, 0)
	return buf.String()
}

func writeJSON(buf *bytes.Buffer, v jVal, pad string, level int) {
	switch v.k {
	case kindNull, kindUndef:
		buf.WriteString("null")
	case kindBool:
		if v.b {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case kindNum:
		buf.Write(scalarJSON(finite(v.n)))
	case kindStr:
		buf.Write(scalarJSON(v.s))
	case kindArr:
		writeArr(buf, v.a, pad, level)
	case kindObj:
		writeObj(buf, v.o, pad, level)
	default:
		buf.WriteString("null")
	}
}

func finite(n float64) any {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return nil
	}
	return n
}

func scalarJSON(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return []byte("null")
	}
	out := buf.Bytes()
	if len(out) > 0 && out[len(out)-1] == '\n' {
		out = out[:len(out)-1]
	}
	return out
}

func writeArr(buf *bytes.Buffer, items []jVal, pad string, level int) {
	if len(items) == 0 {
		buf.WriteString("[]")
		return
	}
	if pad == "" {
		buf.WriteByte('[')
		for i, el := range items {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeJSON(buf, el, "", 0)
		}
		buf.WriteByte(']')
		return
	}
	buf.WriteString("[\n")
	for i, el := range items {
		if i > 0 {
			buf.WriteString(",\n")
		}
		writeIndent(buf, pad, level+1)
		writeJSON(buf, el, pad, level+1)
	}
	buf.WriteByte('\n')
	writeIndent(buf, pad, level)
	buf.WriteByte(']')
}

func writeObj(buf *bytes.Buffer, m *ordMap, pad string, level int) {
	if m == nil || len(m.keys) == 0 {
		buf.WriteString("{}")
		return
	}
	if pad == "" {
		buf.WriteByte('{')
		for i, key := range m.keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.Write(scalarJSON(key))
			buf.WriteByte(':')
			writeJSON(buf, m.vals[key], "", 0)
		}
		buf.WriteByte('}')
		return
	}
	buf.WriteString("{\n")
	for i, key := range m.keys {
		if i > 0 {
			buf.WriteString(",\n")
		}
		writeIndent(buf, pad, level+1)
		buf.Write(scalarJSON(key))
		buf.WriteString(": ")
		writeJSON(buf, m.vals[key], pad, level+1)
	}
	buf.WriteByte('\n')
	writeIndent(buf, pad, level)
	buf.WriteByte('}')
}

func writeIndent(buf *bytes.Buffer, pad string, level int) {
	for i := 0; i < level; i++ {
		buf.WriteString(pad)
	}
}
