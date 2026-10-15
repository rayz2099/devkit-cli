package codeup

// 命令种类与 TypeScript 对齐。doctor 和 version 是 Go 侧额外入口，不读业务默认值。
const (
	kindHelp     = "help"
	kindFish     = "completion-fish"
	kindComplete = "complete"
	kindInit     = "init"
	kindRepos    = "repos"
	kindPush     = "push"
	kindCrList   = "cr-list"
	kindCrGet    = "cr-get"
	kindCrCreate = "cr-create"
	kindCrMerge  = "cr-merge"
	kindHookList = "webhook-list"
	kindDoctor   = "doctor"
	kindVersion  = "version"
)

const (
	audienceHuman = "human"
	audienceAgent = "agent"
)

// MergeTypes 必须和 OpenAPI mergeType 逐字一致，漏一个 CLI 就会编造策略。
var MergeTypes = []string{"squash", "ff-only", "no-fast-forward", "rebase"}

// Profile 是一份云效组织连接。token 只来自配置文件，不从环境变量补。
type Profile struct {
	Name  string
	URL   string
	Token string
}

// FileCfg 是配置文件的全部内容。缺字段直接失败，不补默认组织。
type FileCfg struct {
	DefaultProfile string
	Profiles       []Profile
}

// Runtime 是一次命令选中的连接。API 根固定，不随配置改主机。
type Runtime struct {
	Audience string
	Profile  Profile
	OrgID    string
	Token    string
	APIBase  string
}

// Repo 是仓库列表里的一行。字段名要进 agent JSON，不能改成蛇形。
type Repo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	PathNs    string `json:"pathNs"`
	WebURL    string `json:"webUrl"`
	DefBranch string `json:"defBranch"`
}

// ChangeRequest 是变更请求。crUrl 由 webUrl 拼出来，响应里没有这个字段。
type ChangeRequest struct {
	LocalID     string `json:"localId"`
	Title       string `json:"title"`
	State       string `json:"state"`
	SrcBranch   string `json:"srcBranch"`
	TgtBranch   string `json:"tgtBranch"`
	Author      string `json:"author"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	WebURL      string `json:"webUrl"`
	CrURL       string `json:"crUrl"`
	Description string `json:"description"`
}

// Webhook 是仓库钩子。secret 默认打码，--show-secrets 才原样输出。
type Webhook struct {
	ID                  string `json:"id"`
	URL                 string `json:"url"`
	SecretToken         string `json:"secretToken"`
	PushEvents          bool   `json:"pushEvents"`
	MergeRequestsEvents bool   `json:"mergeRequestsEvents"`
	TagPushEvents       bool   `json:"tagPushEvents"`
	NoteEvents          bool   `json:"noteEvents"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}

// Cmd 是解析后的 argv。扁平放在一处，避免每个子命令再套一层参数。
type Cmd struct {
	Kind        string
	Topic       string
	Audience    string
	Profile     string
	Config      string
	Search      string
	Page        int
	PerPage     int
	Remote      string
	Branch      string
	HasBranch   bool
	Repo        string
	State       string
	Source      *string
	Target      *string
	Title       string
	Body        *string
	BodyFile    *string
	LocalID     string
	MergeType   string
	Message     *string
	ShowSecrets bool
	Tokens      []string
	Current     string
}
