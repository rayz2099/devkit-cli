package issue

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// route 显式绑定后端白名单，不提供任意 HTTP 逃生口，防止误用管理接口。
type route struct {
	group, action, method, path, help string
	args                              int
	query                             string
	version, tree, confirm            bool
}

var routes = []route{
	{group: "items", action: "list", method: "GET", path: "work-items", help: "搜索根事项（items）及完整子树（treeItems），不是平铺匹配", query: "q status ownerId participantId collaboratorId type priority categoryId tagId issue from to sort offset limit"},
	{group: "items", action: "get", method: "GET", path: "work-items/%s", args: 1, help: "读取工作项"},
	{group: "items", action: "children", method: "GET", path: "work-items/%s/children", args: 1, help: "读取直接子节点"},
	{group: "items", action: "preview", method: "GET", path: "work-items/%s/preview", args: 1, help: "预览子树及影响"},
	{group: "items", action: "create", method: "POST", path: "work-items", help: "创建事项/分工/任务；子节点需根 expectedTreeVersion"},
	{group: "items", action: "update", method: "PATCH", path: "work-items/%s", args: 1, version: true, help: "更新 {expectedVersion,changes}，ITEM 流转使用 transitionTarget"},
	{group: "items", action: "move", method: "POST", path: "work-items/%s/move", args: 1, version: true, tree: true, help: "移动 {expectedVersion,expectedTreeVersion,targetParentId}"},
	{group: "items", action: "delete", method: "DELETE", path: "work-items/%s", args: 1, version: true, tree: true, confirm: true, help: "删除子树，需要 --confirm 及两个版本"},
	{group: "dashboard", action: "my-work", method: "GET", path: "dashboard/my-work", help: "后端我的工作（含协作树）", query: "q status ownerId participantId collaboratorId type priority categoryId tagId issue from to sort offset limit"},
	{group: "dashboard", action: "owned", method: "GET", path: "dashboard/owned", help: "后端我负责的树", query: "q status ownerId participantId collaboratorId type priority categoryId tagId issue from to sort offset limit"},
	{group: "dashboard", action: "issues", method: "GET", path: "dashboard/issues", help: "后端与我有关的异常树；scope=WORKSPACE 显式扩大范围", query: "q status ownerId participantId collaboratorId type priority categoryId tagId issue from to sort offset limit scope"},
	{group: "activities", action: "list", method: "GET", path: "work-items/%s/activities", args: 1, help: "工作项时间线，含评论/进展/系统历史，保留 cursor", query: "scope cursor limit"},
	{group: "activities", action: "feed", method: "GET", path: "activities", help: "全局活动流，默认 RELATED", query: "scope cursor limit kind q actorId from to"},
	{group: "activities", action: "create", method: "POST", path: "work-items/%s/activities", args: 1, help: "发布 {type:COMMENT|PROGRESS_UPDATE,content}"},
	{group: "activities", action: "update", method: "PATCH", path: "activities/%s", args: 1, version: true, help: "编辑活动并保留修订 {expectedVersion,content}"},
	{group: "activities", action: "revisions", method: "GET", path: "activities/%s/revisions", args: 1, help: "读取活动修订历史"},
	{group: "knowledge", action: "get", method: "GET", path: "work-items/%s/knowledge", args: 1, help: "读取根事项资料、结论、置顶和历史"},
	{group: "knowledge", action: "update", method: "PATCH", path: "work-items/%s/knowledge", args: 1, version: true, help: "资料命令 action=save|pin|unpin；初始 expectedVersion=0"},
	{group: "relations", action: "list", method: "GET", path: "work-items/%s/relations", args: 1, help: "读取关联"},
	{group: "relations", action: "create", method: "POST", path: "work-items/%s/relations", args: 1, help: "新增关联"},
	{group: "relations", action: "update", method: "PATCH", path: "work-items/%s/relations/%s", args: 2, version: true, help: "编辑关联备注 {expectedVersion,note}"},
	{group: "relations", action: "delete", method: "DELETE", path: "work-items/%s/relations/%s", args: 2, version: true, confirm: true, help: "删除关联（不删除事项），需 --confirm"},
	{group: "successors", action: "create", method: "POST", path: "work-items/%s/successors", args: 1, help: "创建后续事项"},
	{group: "templates", action: "list", method: "GET", path: "templates", help: "列出模板，all=true 包含停用模板", query: "all"},
	{group: "templates", action: "save", method: "POST", path: "templates", help: "保存模板；更新时 body.id 和 expectedVersion 必填"},
	{group: "templates", action: "update", method: "PATCH", path: "templates/%s", args: 1, version: true, help: "启停模板 {expectedVersion,active}"},
	{group: "templates", action: "preview", method: "POST", path: "templates/%s/preview", args: 1, help: "预览模板 {startDate}"},
	{group: "templates", action: "instantiate", method: "POST", path: "templates/%s/instantiate", args: 1, help: "实例化模板"},
	{group: "trash", action: "list", method: "GET", path: "trash/batches", help: "列出删除批次"},
	{group: "trash", action: "get", method: "GET", path: "trash/batches/%s", args: 1, help: "读取删除批次"},
	{group: "trash", action: "restore", method: "POST", path: "trash/batches/%s/restore", args: 1, version: true, help: "按批次恢复 {expectedVersion}"},
	{group: "workspace", action: "snapshot", method: "GET", path: "snapshot", help: "空间元数据及全部未删除项"},
	{group: "workspace", action: "workflow", method: "POST", path: "workspace/workflows", help: "发布流程；基于 sourceId 更新需 expectedVersion"},
	{group: "workspace", action: "workgroups", method: "POST", path: "workspace/workgroups", version: true, help: "分工目录命令 action=create|schedule|reorder"},
	{group: "workspace", action: "dictionary", method: "POST", path: "workspace/dictionaries", help: "保存 category/tag，更新 id 时需 expectedVersion；不管理 role"},
}

func (r route) resolve(args []string) (string, error) {
	vals := make([]any, len(args))
	for i, id := range args {
		if strings.TrimSpace(id) == "" || strings.ContainsAny(id, "/\\?#%") || id == "." || id == ".." {
			return "", errors.New("ID 不能含路径或查询字符")
		}
		vals[i] = url.PathEscape(id)
	}
	if len(vals) == 0 {
		return r.path, nil
	}
	return fmt.Sprintf(r.path, vals...), nil
}

// validateBody 只校验安全边界和并发前提；复杂字段交给服务端的严格 schema。
func (r route) validateBody(body map[string]any, confirm bool) error {
	ver := r.version || ((r.action == "save" || r.action == "dictionary") && body["id"] != nil) || (r.action == "workflow" && body["sourceId"] != nil)
	if ver {
		if err := requireVersion(body, "expectedVersion"); err != nil {
			return err
		}
	}
	if r.tree {
		if err := requireVersion(body, "expectedTreeVersion"); err != nil {
			return err
		}
	}
	if r.confirm && !confirm {
		return errors.New("删除必须显式传 --confirm；不会交互确认")
	}
	if r.group == "items" && r.action == "delete" {
		body["confirmed"] = true
	}
	if body["confirmUnfinishedDescendants"] == true {
		if !confirm {
			return errors.New("结束未完成后代必须显式传 --confirm")
		}
		if err := requireVersion(body, "expectedTreeVersion"); err != nil {
			return err
		}
	}
	if r.group == "workspace" && r.action == "dictionary" && body["kind"] != "category" && body["kind"] != "tag" {
		return errors.New("字典仅支持 category/tag")
	}
	return nil
}

func requireVersion(body map[string]any, key string) error {
	n, ok := body[key].(float64)
	if !ok || n < 0 || n != float64(int64(n)) {
		return fmt.Errorf("JSON body 必须含非负整数 %s；冲突不能重试覆盖", key)
	}
	return nil
}
