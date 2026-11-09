package issue

import (
	"errors"
	"fmt"
	"sort"
)

// personalGroups 保留根事项作为上下文，但只将当前负责人的匹配节点计入个人工作量。
func personalGroups(all, selected []any) ([]any, error) {
	roots := map[string]map[string]any{}
	for _, v := range all {
		item := v.(map[string]any)
		if item["type"] == "ITEM" && item["parentId"] == nil {
			id, _ := item["id"].(string)
			roots[id] = item
		}
	}
	byRoot := map[string][]any{}
	for _, v := range selected {
		item := v.(map[string]any)
		id, ok := item["rootId"].(string)
		if !ok || roots[id] == nil {
			return nil, fmt.Errorf("工作项 %v 缺少有效顶层事项", item["id"])
		}
		byRoot[id] = append(byRoot[id], item)
	}
	ids := make([]string, 0, len(byRoot))
	for id := range byRoot {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	groups := make([]any, 0, len(ids))
	for _, id := range ids {
		items := byRoot[id]
		groups = append(groups, map[string]any{"root": roots[id], "items": items, "count": len(items)})
	}
	return groups, nil
}

// snapshotItems 严格检查结构，避免快照损坏时漏掉高影响操作的后代。
func snapshotItems(raw any) (map[string]any, []any, error) {
	state, ok := raw.(map[string]any)
	if !ok {
		return nil, nil, errors.New("snapshot 格式错误")
	}
	all, ok := state["items"].([]any)
	if !ok {
		return nil, nil, errors.New("snapshot 缺少 items")
	}
	seen := map[string]bool{}
	for _, v := range all {
		item, ok := v.(map[string]any)
		if !ok {
			return nil, nil, errors.New("snapshot 工作项格式错误")
		}
		id, ok := item["id"].(string)
		if !ok || id == "" || seen[id] {
			return nil, nil, errors.New("snapshot 工作项 ID 无效或重复")
		}
		seen[id] = true
	}
	return state, all, nil
}
