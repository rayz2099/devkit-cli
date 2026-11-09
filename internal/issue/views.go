package issue

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// addViews 个人视图从完整快照筛选节点，不把搜索接口返回的根或 treeItems 当作匹配节点。
func (r *runner) addViews(root, items *cobra.Command) {
	items.AddCommand(r.doneCommand())
	me := &cobra.Command{Use: "me", Short: "个人任务列表及事项分组；默认只看当前负责人"}
	root.AddCommand(me)
	for _, view := range []string{"list", "groups", "doing", "due", "overdue", "done"} {
		var owner string
		var days int
		cmd := &cobra.Command{Use: view, Args: cobra.NoArgs}
		if view == "list" {
			cmd.Aliases = []string{"unfinished"}
			cmd.Short = "自己的未完成工作项（含暂停跟踪项）"
		}
		if view == "groups" {
			cmd.Short = "按顶层事项分组展示自己的未完成工作项"
		}
		cmd.Flags().StringVar(&owner, "owner", "", "显式指定负责人 ID；默认 snapshot.user.id")
		cmd.Flags().IntVar(&days, "days", 3, "due 的临期天数，含今天及第 N 天")
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			if days < 0 || days > 365 {
				return errors.New("days 必须在 0..365")
			}
			cli, err := r.client()
			if err != nil {
				return err
			}
			raw, err := cli.Do(cmd.Context(), "GET", "snapshot", nil, nil)
			if err != nil {
				return err
			}
			state, ok := raw.(map[string]any)
			if !ok {
				return errors.New("snapshot 格式错误")
			}
			user, ok := state["user"].(map[string]any)
			if !ok {
				return errors.New("snapshot 缺少 user")
			}
			if owner == "" {
				owner, _ = user["id"].(string)
			}
			if owner == "" {
				return errors.New("snapshot 缺少当前用户 ID")
			}
			space, ok := state["workspace"].(map[string]any)
			if !ok {
				return errors.New("snapshot 缺少 workspace")
			}
			zone, ok := space["timezone"].(string)
			if !ok || zone == "" {
				return errors.New("snapshot 缺少空间时区")
			}
			loc, err := time.LoadLocation(zone)
			if err != nil {
				return errors.New("无法加载空间时区")
			}
			now := r.opts.Now().In(loc)
			day := now.Format("2006-01-02")
			end := now.AddDate(0, 0, days).Format("2006-01-02")
			all, ok := state["items"].([]any)
			if !ok {
				return errors.New("snapshot 缺少 items")
			}
			out := []any{}
			for _, value := range all {
				item, ok := value.(map[string]any)
				if !ok {
					return errors.New("snapshot 工作项格式错误")
				}
				if item["ownerId"] != owner {
					continue
				}
				status, _ := item["status"].(string)
				active := item["trackingActive"] == true
				match := false
				switch view {
				case "list", "groups":
					match = status != "COMPLETED" && status != "CANCELLED"
				case "doing":
					match = status == "IN_PROGRESS" && active
				case "done":
					match = status == "COMPLETED"
				case "due":
					date, _ := item["deadlineDate"].(string)
					match = active && date != "" && date >= day && date <= end
				case "overdue":
					issues, _ := item["selfIssues"].([]any)
					for _, v := range issues {
						issue, ok := v.(map[string]any)
						if ok && issue["type"] == "OVERDUE" {
							match = true
						}
					}
				}
				if match {
					out = append(out, item)
				}
			}
			result := map[string]any{"view": view, "ownerId": owner, "items": out, "count": len(out), "timezone": zone}
			if view == "groups" {
				groups, err := personalGroups(all, out)
				if err != nil {
					return err
				}
				result["groups"] = groups
			}
			return r.personalOutput(result)
		}
		me.AddCommand(cmd)
	}
	items.AddCommand(&cobra.Command{Use: "subtree ID", Short: "从 snapshot 读取指定节点及全部后代（不分页）", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		spec := route{path: "work-items/%s"}
		if _, err := spec.resolve(args); err != nil {
			return err
		}
		cli, err := r.client()
		if err != nil {
			return err
		}
		raw, err := cli.Do(cmd.Context(), "GET", "snapshot", nil, nil)
		if err != nil {
			return err
		}
		state, ok := raw.(map[string]any)
		if !ok {
			return errors.New("snapshot 格式错误")
		}
		all, ok := state["items"].([]any)
		if !ok {
			return errors.New("snapshot 缺少 items")
		}
		byParent := map[string][]map[string]any{}
		var seed map[string]any
		for _, v := range all {
			item, ok := v.(map[string]any)
			if !ok {
				return errors.New("snapshot 工作项格式错误")
			}
			if item["id"] == args[0] {
				seed = item
			}
			parent, _ := item["parentId"].(string)
			byParent[parent] = append(byParent[parent], item)
		}
		if seed == nil {
			return fmt.Errorf("工作项不存在或在回收站：%s", args[0])
		}
		out := []any{}
		queue := []map[string]any{seed}
		seen := map[string]bool{}
		for len(queue) > 0 {
			item := queue[0]
			queue = queue[1:]
			id, ok := item["id"].(string)
			if !ok || seen[id] {
				return errors.New("snapshot 子树包含无效 ID 或环")
			}
			seen[id] = true
			out = append(out, item)
			queue = append(queue, byParent[id]...)
		}
		return r.output(map[string]any{"items": out, "count": len(out)})
	}})
}
