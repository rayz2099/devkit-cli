package codews

// Repo 是 project.yml 解析后的一个仓库。
// 路径和主分支都从 catalog 来，避免 config.json 各自写一份会漂的绝对路径。
type Repo struct {
	Name     string
	Path     string
	Group    string
	Branch   string
	Checkout string
	Merge    string
	Desc     string
}

// Profile 是一组仓库加上 agents 模板目录。
type Profile struct {
	Name   string
	Agents string
	Repos  []Repo
}

// Cfg 是执行期配置。Profiles 的顺序跟 config.json 里的键顺序一致。
type Cfg struct {
	WsRoot   string
	Base     string
	Remote   string
	InitTpl  string
	Projects []Repo
	Profiles map[string]Profile
	Order    []string
}

// Cmd 是一条待执行的 git 命令。Ignored 命中 stderr 时这条失败可以跳过。
type Cmd struct {
	Repo    string
	Branch  string
	Step    string
	Cwd     string
	Args    []string
	Ignored []string
}

// Exit 让 main 按指定码退出。Msg 为空时不往 stderr 写，避免 doctor 的 stdout 协议被前缀污染。
type Exit struct {
	Code int
	Msg  string
}

func (e *Exit) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return "exit"
}
