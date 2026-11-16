package issue

import (
	"errors"
	"fmt"
	"strings"
)

type routeMode int

const (
	// modeRead 仅发送 target 或空对象，不接收 --body-file。
	modeRead routeMode = iota
	// modeSearch 支持 --query 转结构化筛选体或直接传 --body-file，默认发送 {}。
	modeSearch
	// modeWrite 必须通过 --body-file 提供完整请求体，CLI 仅注入路径 ID 与确认边界。
	modeWrite
)

// route 显式绑定后端 apiOperations 白名单，所有 Token 接口统一使用 POST /api/<operation>。
type route struct {
	group, action, path, help string
	mode                      routeMode
	args                      int
	query                     string
	view                      string
	version, tree, confirm    bool
}

var routes = []route{
	{group: "items", action: "list", path: "work-items/search", mode: modeSearch, help: "搜索根事项（items）及完整子树（treeItems），不是平铺匹配", query: "q status ownerId participantId collaboratorId type priority categoryId tagId issue scope from to offset limit"},
	{group: "items", action: "get", path: "work-items/detail", mode: modeRead, args: 1, help: "读取工作项"},
	{group: "items", action: "children", path: "work-items/children", mode: modeRead, args: 1, help: "读取直接子节点"},
	{group: "items", action: "preview", path: "work-items/preview", mode: modeRead, args: 1, help: "预览子树及影响"},
	{group: "items", action: "create", path: "work-items/create", mode: modeWrite, help: "创建事项/分工/任务；子节点需 concurrency.expectedTreeVersion"},
	{group: "items", action: "update", path: "work-items/edit", mode: modeWrite, args: 1, version: true, help: "更新 {concurrency:{expectedVersion},changes}，ITEM 流转使用 transition"},
	{group: "items", action: "move", path: "work-items/move", mode: modeWrite, args: 1, version: true, tree: true, help: "移动 {destination:{id},concurrency:{expectedVersion,expectedTreeVersion}}"},
	{group: "items", action: "delete", path: "work-items/delete", mode: modeWrite, args: 1, version: true, tree: true, confirm: true, help: "删除子树，需要 --confirm 及两个版本"},
	{group: "dashboard", action: "my-work", path: "dashboard/search", mode: modeSearch, view: "my-work", help: "后端我的工作（含协作树）", query: "q status ownerId participantId collaboratorId type priority categoryId tagId issue scope from to offset limit"},
	{group: "dashboard", action: "owned", path: "dashboard/search", mode: modeSearch, view: "owned", help: "后端我负责的树", query: "q status ownerId participantId collaboratorId type priority categoryId tagId issue scope from to offset limit"},
	{group: "dashboard", action: "issues", path: "dashboard/issues", mode: modeSearch, view: "issues", help: "后端与我有关的异常树；scope=WORKSPACE 显式扩大范围", query: "q status ownerId participantId collaboratorId type priority categoryId tagId issue scope from to offset limit"},
	{group: "activities", action: "list", path: "activities/timeline", mode: modeSearch, args: 1, help: "工作项时间线，含评论/进展/系统历史，保留 cursor", query: "scope cursor limit"},
	{group: "activities", action: "feed", path: "activities/search", mode: modeSearch, help: "全局活动流，默认 RELATED", query: "scope cursor limit kind q actorId from to"},
	{group: "activities", action: "create", path: "activities/create", mode: modeWrite, args: 1, help: "发布 {data:{type:COMMENT|PROGRESS_UPDATE,content}}"},
	{group: "activities", action: "update", path: "activities/edit", mode: modeWrite, args: 1, version: true, help: "编辑活动并保留修订 {concurrency:{expectedVersion},changes:{content}}"},
	{group: "activities", action: "revisions", path: "activities/revisions", mode: modeRead, args: 1, help: "读取活动修订历史"},
	{group: "knowledge", action: "get", path: "knowledge/detail", mode: modeRead, args: 1, help: "读取根事项资料、结论、置顶和历史"},
	{group: "knowledge", action: "update", path: "knowledge/edit", mode: modeWrite, args: 1, version: true, help: "资料命令 {concurrency:{expectedVersion},changes:{action:pin|unpin,activityId}}"},
	{group: "relations", action: "list", path: "relations/search", mode: modeRead, args: 1, help: "读取关联"},
	{group: "relations", action: "create", path: "relations/create", mode: modeWrite, args: 1, help: "新增关联 {data:{...}}"},
	{group: "relations", action: "update", path: "relations/edit", mode: modeWrite, args: 2, version: true, help: "编辑关联备注 {concurrency:{expectedVersion},changes:{note}}"},
	{group: "relations", action: "delete", path: "relations/delete", mode: modeWrite, args: 2, version: true, confirm: true, help: "删除关联（不删除事项），需 --confirm"},
	{group: "successors", action: "create", path: "work-items/create-successor", mode: modeWrite, args: 1, help: "创建后续事项 {data:{...}}"},
	{group: "templates", action: "list", path: "templates/search", mode: modeSearch, help: "列出模板，all=true 包含停用模板", query: "all"},
	{group: "templates", action: "save", path: "templates/save", mode: modeWrite, help: "保存模板；更新时 target.id 和 concurrency.expectedVersion 必填"},
	{group: "templates", action: "update", path: "templates/set-active", mode: modeWrite, args: 1, version: true, help: "启停模板 {concurrency:{expectedVersion},changes:{active}}"},
	{group: "templates", action: "preview", path: "templates/preview", mode: modeWrite, args: 1, help: "预览模板 {data:{startDate}}"},
	{group: "templates", action: "instantiate", path: "templates/instantiate", mode: modeWrite, args: 1, help: "实例化模板 {data:{...},concurrency:{expectedTemplateVersion,expectedWorkgroupVersion}}"},
	{group: "trash", action: "list", path: "trash/search", mode: modeRead, help: "列出删除批次"},
	{group: "trash", action: "get", path: "trash/detail", mode: modeRead, args: 1, help: "读取删除批次"},
	{group: "trash", action: "restore", path: "trash/restore", mode: modeWrite, args: 1, version: true, help: "按批次恢复 {concurrency:{expectedVersion}}"},
	{group: "workspace", action: "snapshot", path: "workspace/snapshot", mode: modeRead, help: "空间元数据及全部未删除项"},
	{group: "workspace", action: "account", path: "account/detail", mode: modeRead, help: "当前 Token 绑定的账号与空间信息"},
	{group: "workspace", action: "workflow", path: "workflows/publish", mode: modeWrite, help: "发布流程；基于 target.id 更新需 concurrency.expectedVersion"},
	{group: "workspace", action: "workgroups", path: "workgroups/configure", mode: modeWrite, version: true, help: "分工目录命令 {concurrency:{expectedVersion},changes:{action:...}}"},
	{group: "workspace", action: "dictionary", path: "dictionaries/save", mode: modeWrite, help: "保存 category/tag，更新 target.id 时需 concurrency.expectedVersion；不管理 role"},
}

func init() {
	// dashboard/issues 路由 path 统一指向 dashboard/search。
	for i := range routes {
		if routes[i].group == "dashboard" && routes[i].action == "issues" {
			routes[i].path = "dashboard/search"
		}
	}
}

// validateIDs 拦截路径穿越与查询字符，防止将异常 ID 注入 target 对象。
func validateIDs(args []string) error {
	for _, id := range args {
		if strings.TrimSpace(id) == "" || strings.ContainsAny(id, "/\\?#%") || id == "." || id == ".." {
			return errors.New("ID 不能含路径或查询字符")
		}
	}
	return nil
}

// injectTarget 将命令行位置参数写入新版契约要求的 target 对象。
func (r route) injectTarget(body map[string]any, args []string) {
	if len(args) == 1 {
		body["target"] = map[string]any{"id": args[0]}
	} else if len(args) == 2 {
		body["target"] = map[string]any{"itemId": args[0], "relationId": args[1]}
	}
}

// validateBody 只校验安全边界和并发前提；复杂字段交给服务端的严格 schema。
func (r route) validateBody(body map[string]any, confirm bool) error {
	ver := r.version || ((r.path == "templates/save" || r.path == "workflows/publish" || r.path == "dictionaries/save") && body["target"] != nil)
	if ver {
		if err := requireConcurrencyVersion(body, "expectedVersion"); err != nil {
			return err
		}
	}
	if r.tree {
		if err := requireConcurrencyVersion(body, "expectedTreeVersion"); err != nil {
			return err
		}
	}
	if r.path == "templates/instantiate" {
		if err := requireConcurrencyVersion(body, "expectedTemplateVersion"); err != nil {
			return err
		}
		if err := requireConcurrencyVersion(body, "expectedWorkgroupVersion"); err != nil {
			return err
		}
	}
	if r.confirm && !confirm {
		return errors.New("删除必须显式传 --confirm；不会交互确认")
	}
	if r.path == "work-items/delete" {
		body["confirmation"] = map[string]any{"confirmed": true}
	}
	if conf, ok := body["confirmation"].(map[string]any); ok && conf["unfinishedDescendants"] == true {
		if !confirm {
			return errors.New("结束未完成后代必须显式传 --confirm")
		}
		if err := requireConcurrencyVersion(body, "expectedTreeVersion"); err != nil {
			return err
		}
	}
	if r.path == "dictionaries/save" {
		data, _ := body["data"].(map[string]any)
		if data == nil || (data["kind"] != "category" && data["kind"] != "tag") {
			return errors.New("字典仅支持 category/tag")
		}
	}
	return nil
}

func requireConcurrencyVersion(body map[string]any, key string) error {
	conc, _ := body["concurrency"].(map[string]any)
	if conc == nil {
		return fmt.Errorf("JSON body 必须含 concurrency.%s；冲突不能重试覆盖", key)
	}
	n, ok := conc[key].(float64)
	if !ok || n < 0 || n != float64(int64(n)) {
		return fmt.Errorf("JSON body 必须含非负整数 concurrency.%s；冲突不能重试覆盖", key)
	}
	return nil
}
