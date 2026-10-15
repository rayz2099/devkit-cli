package rclone

import (
	"regexp"
	"strings"
)

// CryptSecret 是 obscure 之后的一对密钥。连接串里的字段名仍是 password / password2。
type CryptSecret struct {
	Name      string
	Password  string
	Password2 string
}

// Secrets 是一次传输要用的已 obscure 口令。明文只留在配置里。
type Secrets struct {
	Pass  string
	Crypt []CryptSecret
}

// ParsedConn 是连接串拆开后的结果，用来证明特殊字符还能原样解出来。
type ParsedConn struct {
	Name   string
	Config map[string]string
	Path   string
}

var nameRe = regexp.MustCompile(`^:?[\w.+@-]+$`)
var paramRe = regexp.MustCompile(`^[A-Za-z0-9_.]+$`)

// CryptOrder 把 default 放在前面。加密只用第一对，解密才继续试后面的。
func CryptOrder(target Target) []string {
	if !target.HasCrypt {
		return nil
	}
	var head []string
	var rest []string
	for _, item := range target.Crypt {
		if item.Default {
			head = append(head, item.Name)
		} else {
			rest = append(rest, item.Name)
		}
	}
	return append(head, rest...)
}

// EncVal 给连接串的值加单引号。未加引号的 `:` 或 `,` 会被 rclone 截断。
func EncVal(raw string) string {
	return "'" + strings.ReplaceAll(raw, "'", "''") + "'"
}

// BuildRemote 拼 on-the-fly remote。网盘目录名就是 root，不能再套一层 crypt 目录。
func BuildRemote(target Target, secrets Secrets, rel, nameEnc, suffix, pair string) (string, error) {
	folder := trimSlash(target.Root)
	sub := trimSlash(rel)
	if !target.HasCrypt {
		return webdavHead(target, secrets.Pass) + joinDest(folder, sub), nil
	}
	names := CryptOrder(target)
	picked := pair
	if picked == "" && len(names) > 0 {
		picked = names[0]
	}
	if picked == "" {
		return "", fail(1, "targets.%s.crypt requires a default key pair", target.Name)
	}
	wire, ok := findWire(secrets, picked)
	if !ok || wire.Password == "" || wire.Password2 == "" {
		return "", fail(1, "targets.%s.crypt.%s requires key and salt", target.Name, picked)
	}
	web := webdavHead(target, secrets.Pass) + folder
	parts := []string{
		":crypt",
		"remote=" + EncVal(web),
	}
	parts = append(parts, nameFlags(nameEnc, suffix)...)
	parts = append(parts, "password="+EncVal(wire.Password), "password2="+EncVal(wire.Password2))
	head := strings.Join(parts, ",")
	if sub == "" {
		return head + ":", nil
	}
	return head + ":" + sub, nil
}

// BuildPlainRemote 直接走 WebDAV。密文先拉下来，换钥匙只在本地解，避免同一文件下两遍。
func BuildPlainRemote(target Target, pass, rel string) string {
	return webdavHead(target, pass) + joinDest(trimSlash(target.Root), trimSlash(rel))
}

// BuildLocalCrypt 解本地临时文件。文件名是我们起的，必须关文件名加密，否则 crypt 会去找密文名。
func BuildLocalCrypt(folder string, wire CryptSecret, file string) string {
	parts := []string{
		":crypt",
		"remote=" + EncVal(folder),
		"filename_encryption=off",
		"directory_name_encryption=false",
		"suffix=" + EncVal("none"),
		"password=" + EncVal(wire.Password),
		"password2=" + EncVal(wire.Password2),
	}
	return strings.Join(parts, ",") + ":" + file
}

func nameFlags(nameEnc, suffix string) []string {
	if nameEnc == "off" {
		return []string{
			"filename_encryption=off",
			"directory_name_encryption=false",
			"suffix=" + EncVal(suffix),
		}
	}
	return []string{"filename_encryption=standard", "directory_name_encryption=true"}
}

func findWire(secrets Secrets, name string) (CryptSecret, bool) {
	for _, item := range secrets.Crypt {
		if item.Name == name {
			return item, true
		}
	}
	return CryptSecret{}, false
}

func webdavHead(target Target, pass string) string {
	parts := []string{
		":webdav",
		"url=" + EncVal(target.URL),
		"vendor=" + EncVal(target.Vendor),
		"user=" + EncVal(target.User),
		"pass=" + EncVal(pass),
		"pacer_min_sleep=" + EncVal(target.PacerMinSleep),
	}
	return strings.Join(parts, ",") + ":"
}

func joinDest(root, rel string) string {
	base := trimSlash(root)
	sub := trimSlash(rel)
	if sub == "" {
		return base
	}
	if base == "" {
		return sub
	}
	return base + "/" + sub
}

func trimSlash(value string) string {
	return strings.Trim(value, "/")
}

// ParseConn 按 rclone 连接串语法拆开。单测靠它证明引号里的逗号和冒号没有被截断。
func ParseConn(input string) (ParsedConn, error) {
	if input == "" {
		return ParsedConn{}, fail(1, "connection string is empty")
	}
	state := "name"
	prev := 0
	param := ""
	quote := byte(0)
	doubled := false
	name := ""
	config := map[string]string{}
	index := 0
	for ; index < len(input); index++ {
		ch := input[index]
		switch state {
		case "name":
			step, err := takeName(input, index, ch)
			if err != nil {
				return ParsedConn{}, err
			}
			if step.kind == "continue" {
				continue
			}
			name = step.name
			prev = index + 1
			if step.kind == "done" {
				state = "done"
				break
			}
			state = "param"
		case "param":
			step, err := takeParam(input, index, ch, prev)
			if err != nil {
				return ParsedConn{}, err
			}
			if step.kind == "continue" {
				continue
			}
			param = step.param
			prev = index + 1
			if step.kind == "value" {
				state = "value"
				continue
			}
			config[param] = "true"
			if step.kind == "done" {
				state = "done"
				break
			}
			state = "param"
		case "value":
			step := takeValue(input, index, ch, prev, param, config)
			if step.kind == "quoted" {
				quote = ch
				state = "quoted"
				prev = index + 1
				doubled = false
				continue
			}
			if step.kind == "continue" {
				continue
			}
			prev = index + 1
			if step.kind == "done" {
				state = "done"
				break
			}
			state = "param"
		case "quoted":
			if ch == quote {
				state = "after"
			}
		case "after":
			step, err := takeAfter(input, index, ch, prev, quote, doubled, param, config)
			if err != nil {
				return ParsedConn{}, err
			}
			if step.kind == "quote" {
				state = "quoted"
				doubled = true
				continue
			}
			prev = index + 1
			if step.kind == "done" {
				state = "done"
				break
			}
			state = "param"
		}
		if state == "done" {
			break
		}
	}
	if state != "done" {
		return ParsedConn{}, fail(1, "connection string: truncated in %s", state)
	}
	return ParsedConn{Name: name, Config: config, Path: input[index+1:]}, nil
}

type nameStep struct {
	kind string
	name string
}

func takeName(input string, index int, ch byte) (nameStep, error) {
	if index == 0 && ch == ':' {
		return nameStep{kind: "continue"}, nil
	}
	if ch == '/' || ch == '\\' {
		return nameStep{}, fail(1, "connection string: invalid name")
	}
	if ch != ':' && ch != ',' {
		return nameStep{kind: "continue"}, nil
	}
	name := input[:index]
	if !nameRe.MatchString(name) {
		return nameStep{}, fail(1, "connection string: invalid name")
	}
	if ch == ':' {
		return nameStep{kind: "done", name: name}, nil
	}
	return nameStep{kind: "param", name: name}, nil
}

type paramStep struct {
	kind  string
	param string
}

func takeParam(input string, index int, ch byte, prev int) (paramStep, error) {
	if ch != ':' && ch != ',' && ch != '=' {
		if !isParamChar(ch) {
			return paramStep{}, fail(1, "connection string: bad param")
		}
		return paramStep{kind: "continue"}, nil
	}
	param := input[prev:index]
	if param == "" || !paramRe.MatchString(param) {
		return paramStep{}, fail(1, "connection string: bad param")
	}
	if ch == '=' {
		return paramStep{kind: "value", param: param}, nil
	}
	if ch == ':' {
		return paramStep{kind: "done", param: param}, nil
	}
	return paramStep{kind: "flag", param: param}, nil
}

func isParamChar(ch byte) bool {
	if ch >= 'A' && ch <= 'Z' {
		return true
	}
	if ch >= 'a' && ch <= 'z' {
		return true
	}
	if ch >= '0' && ch <= '9' {
		return true
	}
	return ch == '_' || ch == '.'
}

type valueStep struct {
	kind string
}

func takeValue(input string, index int, ch byte, prev int, param string, config map[string]string) valueStep {
	if (ch == '\'' || ch == '"') && index == prev {
		return valueStep{kind: "quoted"}
	}
	if ch != ':' && ch != ',' {
		return valueStep{kind: "continue"}
	}
	config[param] = input[prev:index]
	if ch == ':' {
		return valueStep{kind: "done"}
	}
	return valueStep{kind: "param"}
}

func takeAfter(input string, index int, ch byte, prev int, quote byte, doubled bool, param string, config map[string]string) (valueStep, error) {
	if ch == quote {
		return valueStep{kind: "quote"}, nil
	}
	if ch != ':' && ch != ',' {
		return valueStep{}, fail(1, "connection string: trailing after quote")
	}
	value := input[prev : index-1]
	if doubled {
		value = strings.ReplaceAll(value, string([]byte{quote, quote}), string([]byte{quote}))
	}
	config[param] = value
	if ch == ':' {
		return valueStep{kind: "done"}, nil
	}
	return valueStep{kind: "param"}, nil
}
