package issue

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"git.internal.linran.top/linran/devkit-cli/internal/doctor"
	"github.com/spf13/cobra"
)

// Options 提供隔离测试的注入点，不读取真实配置或访问线上。
type Options struct {
	Out, Err io.Writer
	HTTP     *http.Client
	Load     func(string) (Config, error)
	ReadFile func(string) ([]byte, error)
	Now      func() time.Time
}
type runner struct {
	opts              Options
	cfgPath, audience string
	token             string
}

// NewCommand 将 HTTP、配置和输出解耦，使本机验收不依赖个人凭据。
func NewCommand(opts Options) *cobra.Command {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Err == nil {
		opts.Err = io.Discard
	}
	if opts.Load == nil {
		opts.Load = LoadConfig
	}
	if opts.ReadFile == nil {
		opts.ReadFile = os.ReadFile
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	r := &runner{opts: opts}
	root := &cobra.Command{Use: "issue-cli", Short: "单 URL/Token 的个人工作项客户端", SilenceErrors: true, SilenceUsage: true}
	root.SetOut(opts.Out)
	root.SetErr(opts.Err)
	root.PersistentFlags().StringVar(&r.cfgPath, "config", "", "只读配置路径（默认 XDG_CONFIG_HOME/issue-cli/config.json）")
	root.PersistentFlags().StringVar(&r.audience, "audience", "human", "human 或 agent（JSON）")
	root.PersistentPreRunE = func(*cobra.Command, []string) error {
		if r.audience != "human" && r.audience != "agent" {
			return errors.New("audience 必须是 human 或 agent")
		}
		return nil
	}
	root.AddCommand(&cobra.Command{Use: "version", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error { return r.output(map[string]any{"version": "dev"}) }})
	root.AddCommand(&cobra.Command{Use: "doctor", Short: "只检查本地配置，不联网、不创建", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		_, err := r.opts.Load(r.cfgPath)
		if err != nil {
			return err
		}
		if r.audience == "agent" {
			return r.output(map[string]any{"config": "ok"})
		}
		return doctor.Write(r.opts.Out, []doctor.Item{{Name: "config"}})
	}})
	groups := map[string]*cobra.Command{}
	for _, spec := range routes {
		group := groups[spec.group]
		if group == nil {
			group = &cobra.Command{Use: spec.group}
			groups[spec.group] = group
			root.AddCommand(group)
		}
		group.AddCommand(r.resourceCommand(spec))
	}
	// 状态与普通更新共用并发和确认边界，不复制流程判断。
	status := route{group: "items", action: "transition", path: "work-items/edit", mode: modeWrite, args: 1, version: true, help: "状态流转；ITEM 使用 {concurrency:{expectedVersion},changes:{},transition:{type:STATE|MACRO,id}}"}
	groups["items"].AddCommand(r.resourceCommand(status))
	r.addViews(root, groups["items"])
	configureCompletion(root)
	return root
}

func (r *runner) client() (*Client, error) {
	cfg, err := r.opts.Load(r.cfgPath)
	if err != nil {
		return nil, err
	}
	r.token = cfg.Token
	return NewClient(cfg, r.opts.HTTP)
}

func (r *runner) resourceCommand(spec route) *cobra.Command {
	var file string
	var query []string
	var confirm bool
	use := spec.action
	for i := 0; i < spec.args; i++ {
		use += " ID"
	}
	cmd := &cobra.Command{Use: use, Short: spec.help, Args: cobra.ExactArgs(spec.args)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := validateIDs(args); err != nil {
			return err
		}
		var body map[string]any
		switch spec.mode {
		case modeRead:
			body = map[string]any{}
		case modeSearch:
			if file != "" && len(query) > 0 {
				return errors.New("--body-file 与 --query 不能同时使用")
			}
			if file != "" {
				var err error
				body, err = r.readBodyFile(file)
				if err != nil {
					return err
				}
			} else if len(query) > 0 {
				var err error
				body, err = queryToBody(spec, query)
				if err != nil {
					return err
				}
			} else {
				body = map[string]any{}
			}
			if spec.view != "" && body["view"] == nil {
				body["view"] = spec.view
			}
		case modeWrite:
			if file == "" {
				return errors.New("写入/预览命令必须提供 --body-file JSON 文件")
			}
			var err error
			body, err = r.readBodyFile(file)
			if err != nil {
				return err
			}
			if err := spec.validateBody(body, confirm); err != nil {
				return err
			}
			if spec.action == "transition" && body["transition"] == nil {
				changes, ok := body["changes"].(map[string]any)
				if !ok || changes["status"] == nil {
					return errors.New("transition 需要 transition，非 ITEM 可用 changes.status")
				}
			}
		}
		spec.injectTarget(body, args)
		cli, err := r.client()
		if err != nil {
			return err
		}
		out, err := cli.Do(cmd.Context(), "POST", spec.path, nil, body)
		if err != nil {
			return err
		}
		return r.output(out)
	}
	if spec.mode != modeRead {
		cmd.Flags().StringVar(&file, "body-file", "", "请求 JSON 文件；字段遵循后端契约")
	}
	if spec.query != "" {
		cmd.Flags().StringArrayVar(&query, "query", nil, "可重复 key=value；允许："+spec.query)
	}
	if spec.mode == modeWrite {
		cmd.Flags().BoolVar(&confirm, "confirm", false, "显式确认删除/终止未完成后代，不交互、不自动重试")
	}
	return cmd
}

func (r *runner) readBodyFile(file string) (map[string]any, error) {
	raw, err := r.opts.ReadFile(file)
	if err != nil {
		return nil, errors.New("无法读取 body 文件")
	}
	if len(raw) > 200000 {
		return nil, errors.New("JSON body 超过服务端 200000 字节限制")
	}
	var body map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&body); err != nil || body == nil {
		return nil, errors.New("body 必须是 JSON 对象")
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("body 存在多余内容")
	}
	return body, nil
}

// queryToBody 将 CLI 扁平 --query 参数映射为新版 POST 搜索契约的 filter/page 结构。
func queryToBody(spec route, entries []string) (map[string]any, error) {
	seen := map[string]bool{}
	body := map[string]any{}
	filter := map[string]any{}
	page := map[string]any{}
	for _, entry := range entries {
		key, val, ok := strings.Cut(entry, "=")
		if !ok || !strings.Contains(" "+spec.query+" ", " "+key+" ") || key == "" {
			return nil, fmt.Errorf("不支持的 query 参数 %q", key)
		}
		if seen[key] {
			return nil, fmt.Errorf("query 参数不能重复：%s", key)
		}
		seen[key] = true
		switch key {
		case "limit", "offset":
			n, err := strconv.Atoi(val)
			if err != nil || n < 0 || (key == "limit" && (n < 1 || n > 100)) {
				return nil, fmt.Errorf("%s 必须是整数（limit 1..100，offset >=0）", key)
			}
			page[key] = n
		case "cursor":
			if val == "" {
				return nil, errors.New("cursor 不能为空")
			}
			page["cursor"] = val
		case "all":
			if val == "true" {
				filter["all"] = true
			} else if val == "false" {
				filter["all"] = false
			} else {
				return nil, errors.New("all 必须是 true 或 false")
			}
		case "status":
			filter["statuses"] = strings.Split(val, ",")
		case "tagId":
			filter["tagIds"] = strings.Split(val, ",")
		case "scope":
			if spec.path == "activities/timeline" {
				body["scope"] = val
			} else {
				filter["scope"] = val
			}
		default:
			filter[key] = val
		}
	}
	if len(filter) > 0 {
		body["filter"] = filter
	}
	if len(page) > 0 {
		body["page"] = page
	}
	return body, nil
}

// errorAudience 在 Cobra 查找子命令前确定错误格式，未知命令也必须遵守 agent 协议。
func errorAudience(args []string) string {
	audience := "human"
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "--audience=") {
			audience = strings.TrimPrefix(arg, "--audience=")
			continue
		}
		switch arg {
		case "--audience":
			if i+1 < len(args) {
				i++
				audience = args[i]
			}
		case "--config", "--body-file", "--query", "--owner", "--days":
			i++
		}
	}
	return audience
}

// Execute 统一 stdout/stderr；agent 错误也是 JSON，正常结果不混入诊断信息。
func Execute(args []string, opts Options) int {
	root := NewCommand(opts)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		audience := errorAudience(args)
		dst := opts.Err
		if dst == nil {
			dst = io.Discard
		}
		apiErr := &APIError{Code: "CLIENT_ERROR", Message: err.Error()}
		var remote *APIError
		if errors.As(err, &remote) {
			apiErr = remote
		}
		if audience == "agent" {
			_ = json.NewEncoder(dst).Encode(map[string]any{"error": apiErr})
		} else {
			_, _ = fmt.Fprintln(dst, apiErr.Error())
		}
		return 1
	}
	return 0
}
