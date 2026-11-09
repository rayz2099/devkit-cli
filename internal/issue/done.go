package issue

import (
	"errors"
	"fmt"
	"math"

	"github.com/spf13/cobra"
)

// doneCommand 读取版本和生效流程后仅提交一次，冲突必须由使用者重新评估。
func (r *runner) doneCommand() *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{Use: "done ID", Short: "完成工作项；未完后代须 --confirm，事项不跳过流程", Args: cobra.ExactArgs(1)}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "确认暂停未完成后代跟踪；先 preview 校验树版本")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		spec := route{path: "work-items/%s"}
		path, err := spec.resolve(args)
		if err != nil {
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
		state, all, err := snapshotItems(raw)
		if err != nil {
			return err
		}
		byID := map[string]map[string]any{}
		for _, v := range all {
			item := v.(map[string]any)
			byID[item["id"].(string)] = item
		}
		item := byID[args[0]]
		if item == nil {
			return fmt.Errorf("工作项不存在或在回收站：%s", args[0])
		}
		if item["status"] == "COMPLETED" {
			return r.output(item)
		}
		ver, err := versionField(item, "version")
		if err != nil {
			return err
		}
		body := map[string]any{"expectedVersion": ver, "changes": map[string]any{}}
		switch item["type"] {
		case "ITEM":
			target, err := completionTarget(state, item)
			if err != nil {
				return err
			}
			body["transitionTarget"] = target
		case "WORKGROUP", "TASK":
			body["changes"] = map[string]any{"status": "COMPLETED"}
		default:
			return errors.New("未知工作项类型")
		}
		count, err := unfinishedDescendants(args[0], all, byID)
		if err != nil {
			return err
		}
		if count > 0 && !confirm {
			return &APIError{Code: "INCOMPLETE_DESCENDANTS_CONFIRMATION_REQUIRED", Message: "仍有未完成后代，完成将暂停子树跟踪；确认后使用 --confirm", Details: map[string]any{"count": count}}
		}
		if confirm {
			rootID, _ := item["rootId"].(string)
			root := byID[rootID]
			if root == nil {
				return errors.New("snapshot 缺少根事项")
			}
			treeVer, err := versionField(root, "treeVersion")
			if err != nil {
				return err
			}
			raw, err := cli.Do(cmd.Context(), "GET", path+"/preview", nil, nil)
			if err != nil {
				return err
			}
			preview, ok := raw.(map[string]any)
			if !ok {
				return errors.New("preview 格式错误")
			}
			pv, err := versionField(preview, "treeVersion")
			if err != nil {
				return err
			}
			if treeVer != pv {
				return &APIError{Code: "CONFIRMATION_STALE", Message: "工作结构已变化，请重新查询并确认"}
			}
			body["confirmUnfinishedDescendants"] = true
			body["expectedTreeVersion"] = pv
		}
		out, err := cli.Do(cmd.Context(), "PATCH", path, nil, body)
		if err != nil {
			return err
		}
		return r.output(out)
	}
	return cmd
}

// versionField 不猜测缺失版本，避免弱化服务端乐观锁。
func versionField(item map[string]any, key string) (float64, error) {
	n, ok := item[key].(float64)
	if !ok || n < 0 || math.Trunc(n) != n {
		return 0, fmt.Errorf("响应缺少有效 %s", key)
	}
	return n, nil
}

// unfinishedDescendants 沿父链检查实际后代，而非按 rootId 错把兄弟工作项计入影响。
func unfinishedDescendants(id string, all []any, byID map[string]map[string]any) (int, error) {
	count := 0
	for _, v := range all {
		item := v.(map[string]any)
		if item["id"] == id {
			continue
		}
		seen := map[string]bool{item["id"].(string): true}
		cur := item
		for cur["parentId"] != nil {
			parent, ok := cur["parentId"].(string)
			if !ok || byID[parent] == nil || seen[parent] {
				return 0, errors.New("snapshot 父链缺失或存在环")
			}
			seen[parent] = true
			if parent == id {
				switch item["status"] {
				case "TODO", "IN_PROGRESS":
					count++
				case "COMPLETED", "CANCELLED":
				default:
					return 0, errors.New("snapshot 包含未知状态")
				}
				break
			}
			cur = byID[parent]
		}
	}
	return count, nil
}

// completionMacro 遵守服务 stateMacro 对终态的解释，不把生命周期名当作自定义宏状态 ID。
func completionMacro(flow, state map[string]any) string {
	if id, ok := state["macroStateId"].(string); ok && id != "" {
		return id
	}
	if macros, ok := flow["macroStates"].([]any); ok {
		for _, v := range macros {
			macro, ok := v.(map[string]any)
			if ok && macro["status"] == state["status"] {
				id, _ := macro["id"].(string)
				return id
			}
		}
		return ""
	}
	if state["status"] != "IN_PROGRESS" {
		status, _ := state["status"].(string)
		return status
	}
	// 此命令只解析完成目标，不需要解释进行中状态的历史阶段归属。
	return ""
}

// completionTarget 只选择当前允许的一步完成目标，绝不绕过评估或验收状态。
func completionTarget(state, item map[string]any) (map[string]any, error) {
	flows, ok := state["workflows"].([]any)
	if !ok {
		return nil, errors.New("snapshot 缺少 workflows")
	}
	var flow map[string]any
	for _, v := range flows {
		f, ok := v.(map[string]any)
		if ok && f["id"] == item["workflowId"] {
			flow = f
			break
		}
	}
	if flow == nil {
		return nil, errors.New("找不到事项生效流程")
	}
	states, ok := flow["states"].([]any)
	if !ok {
		return nil, errors.New("流程缺少 states")
	}
	edges, ok := flow["transitions"].([]any)
	if !ok {
		return nil, errors.New("流程缺少 transitions")
	}
	byID := map[string]map[string]any{}
	for _, v := range states {
		s, ok := v.(map[string]any)
		if !ok {
			return nil, errors.New("流程状态格式错误")
		}
		id, ok := s["id"].(string)
		if !ok {
			return nil, errors.New("流程状态缺少 ID")
		}
		byID[id] = s
	}
	current, ok := item["workflowStateId"].(string)
	if !ok || byID[current] == nil {
		return nil, errors.New("事项当前流程状态无效")
	}
	// STATE 和 MACRO 可以指向同一细分状态，按实际状态去重以避免伪歧义。
	targets := map[string]map[string]any{}
	for _, v := range edges {
		edge, ok := v.(map[string]any)
		if !ok {
			return nil, errors.New("流程流转格式错误")
		}
		if edge["from"] != current {
			continue
		}
		id, ok := edge["to"].(string)
		if !ok {
			return nil, errors.New("流程流转缺少目标")
		}
		kind := "STATE" // 服务契约规定省略 targetType 即为细分状态，不是降级路径。
		if value, exists := edge["targetType"]; exists {
			kind, ok = value.(string)
			if !ok {
				return nil, errors.New("流程目标类型无效")
			}
		}
		targetID := id
		if kind == "MACRO" {
			defs, _ := flow["macroDefaults"].(map[string]any)
			if configured, exists := defs[id]; exists {
				targetID, ok = configured.(string)
				if !ok {
					return nil, errors.New("流程默认状态无效")
				}
			} else {
				// 服务的 macroDefault 在未显式配置时使用该宏状态的第一个细分状态。
				targetID = ""
				for _, v := range states {
					s := v.(map[string]any)
					if completionMacro(flow, s) == id {
						targetID = s["id"].(string)
						break
					}
				}
			}
		} else if kind != "STATE" {
			return nil, errors.New("未知流程目标类型")
		}
		target := byID[targetID]
		if target != nil && target["status"] == "COMPLETED" {
			if _, exists := targets[targetID]; !exists {
				targets[targetID] = map[string]any{"type": kind, "id": id}
			}
		}
	}
	if len(targets) == 0 {
		return nil, errors.New("当前状态不允许直接完成；请先按流程使用 items transition")
	}
	if len(targets) > 1 {
		return nil, errors.New("存在多个完成状态；请用 items transition 显式选择")
	}
	for _, target := range targets {
		return target, nil
	}
	return nil, errors.New("未找到完成目标")
}
