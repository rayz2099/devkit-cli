package issue

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const personalSnapshot = `{"user":{"id":"me"},"workspace":{"timezone":"Asia/Shanghai"},"items":[
 {"id":"root","rootId":"root","parentId":null,"type":"ITEM","ownerId":"other","status":"IN_PROGRESS","title":"服务升级","description":"完整根描述"},
 {"id":"mine","rootId":"root","parentId":"root","type":"TASK","ownerId":"me","status":"TODO","title":"发布\n检查","deadlineDate":"2026-11-10","deadlineTime":"18:00","description":"完整任务描述","version":3},
 {"id":"finished","rootId":"root","parentId":"root","type":"TASK","ownerId":"me","status":"COMPLETED","title":"已结束"},
 {"id":"other","rootId":"root","parentId":"root","type":"TASK","ownerId":"other","status":"TODO","title":"别人的任务"}
 ]}`

func TestPersonalTablesAndGroups(t *testing.T) {
	for _, view := range []string{"list", "unfinished", "groups"} {
		t.Run(view, func(t *testing.T) {
			opts, out, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, personalSnapshot) })
			if Execute([]string{"me", view}, opts) != 0 {
				t.Fatal(errOut)
			}
			text := out.String()
			for _, want := range []string{"ID", "类型", "状态", "标题", "截止", "mine", "发布 检查", "2026-11-10 18:00", "共 1 项"} {
				if !strings.Contains(text, want) {
					t.Fatalf("missing %q: %s", want, text)
				}
			}
			for _, absent := range []string{"description", "完整任务描述", "version", "别人的任务", "已结束"} {
				if strings.Contains(text, absent) {
					t.Fatalf("unexpected %q: %s", absent, text)
				}
			}
			if view == "groups" && !strings.Contains(text, "服务升级") {
				t.Fatal(text)
			}
		})
	}
}

func TestPersonalAgentRetainsFieldsAndRootContext(t *testing.T) {
	opts, out, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, personalSnapshot) })
	if Execute([]string{"me", "groups", "--audience", "agent"}, opts) != 0 {
		t.Fatal(errOut)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	items := result["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["description"] != "完整任务描述" {
		t.Fatal(out)
	}
	groups := result["groups"].([]any)
	root := groups[0].(map[string]any)["root"].(map[string]any)
	if root["ownerId"] != "other" || root["description"] != "完整根描述" {
		t.Fatal(out)
	}
}

func TestPersonalEmptyViews(t *testing.T) {
	for _, view := range []string{"list", "groups", "doing", "due", "overdue", "done"} {
		opts, out, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"user":{"id":"me"},"workspace":{"timezone":"UTC"},"items":[]}`)
		})
		if Execute([]string{"me", view}, opts) != 0 {
			t.Fatal(errOut)
		}
		if !strings.Contains(out.String(), "共 0 项") {
			t.Fatal(out)
		}
	}
}

func TestPersonalGroupsRejectMissingRoot(t *testing.T) {
	opts, _, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.ReplaceAll(personalSnapshot, `"rootId":"root"`, `"rootId":"missing"`))
	})
	if Execute([]string{"me", "groups"}, opts) == 0 || !strings.Contains(errOut.String(), "顶层事项") {
		t.Fatal(errOut)
	}
}
