package issue

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

const doneSnapshot = `{"items":[
 {"id":"root","rootId":"root","parentId":null,"type":"ITEM","status":"IN_PROGRESS","workflowId":"flow","workflowStateId":"review","version":4,"treeVersion":9},
 {"id":"group","rootId":"root","parentId":"root","type":"WORKGROUP","status":"IN_PROGRESS","version":6},
 {"id":"task","rootId":"root","parentId":"group","type":"TASK","status":"TODO","version":3},
 {"id":"cancelled","rootId":"root","parentId":"group","type":"TASK","status":"CANCELLED","version":2}
 ],"workflows":[{"id":"flow","states":[{"id":"review","status":"IN_PROGRESS"},{"id":"complete","status":"COMPLETED","macroStateId":"finish"}],"transitions":[{"from":"review","to":"finish","targetType":"MACRO"}],"macroDefaults":{"finish":"complete"}}]}`

func TestDoneUsesVersionsAndSimpleStates(t *testing.T) {
	for _, tc := range []struct {
		id      string
		confirm bool
		version float64
	}{
		{"task", false, 3}, {"group", true, 6}, {"root", true, 4},
	} {
		t.Run(tc.id, func(t *testing.T) {
			var calls []string
			opts, _, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.Method+" "+r.URL.Path)
				switch {
				case r.URL.Path == "/api/snapshot":
					_, _ = io.WriteString(w, doneSnapshot)
				case strings.HasSuffix(r.URL.Path, "/preview"):
					_, _ = io.WriteString(w, `{"treeVersion":9,"items":[],"ancestors":[]}`)
				case r.Method == "PATCH":
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if body["expectedVersion"] != tc.version {
						t.Fatal(body)
					}
					if tc.confirm && (body["expectedTreeVersion"] != float64(9) || body["confirmUnfinishedDescendants"] != true) {
						t.Fatal(body)
					}
					if !tc.confirm && body["expectedTreeVersion"] != nil {
						t.Fatal(body)
					}
					changes := body["changes"].(map[string]any)
					if tc.id == "root" {
						target := body["transitionTarget"].(map[string]any)
						if target["type"] != "MACRO" || target["id"] != "finish" || len(changes) != 0 {
							t.Fatal(body)
						}
					} else if changes["status"] != "COMPLETED" || body["transitionTarget"] != nil {
						t.Fatal(body)
					}
					_, _ = io.WriteString(w, `{"status":"COMPLETED"}`)
				default:
					t.Error(r.URL.Path)
				}
			})
			args := []string{"items", "done", tc.id}
			if tc.confirm {
				args = append(args, "--confirm")
			}
			if Execute(args, opts) != 0 {
				t.Fatal(errOut)
			}
			expected := []string{"GET /api/snapshot"}
			if tc.confirm {
				expected = append(expected, "GET /api/work-items/"+tc.id+"/preview")
			}
			expected = append(expected, "PATCH /api/work-items/"+tc.id)
			if fmt.Sprint(calls) != fmt.Sprint(expected) {
				t.Fatal(calls)
			}
		})
	}
}

func TestDoneSafetyBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, id, doc, preview, code string
		confirm                      bool
	}{
		{name: "unfinished", id: "group", doc: doneSnapshot, code: "INCOMPLETE_DESCENDANTS_CONFIRMATION_REQUIRED"},
		{name: "stale preview", id: "group", doc: doneSnapshot, preview: `{"treeVersion":10}`, confirm: true, code: "CONFIRMATION_STALE"},
		{name: "missing preview version", id: "group", doc: doneSnapshot, preview: `{}`, confirm: true, code: "treeVersion"},
		{name: "no workflow edge", id: "root", doc: strings.Replace(doneSnapshot, `"from":"review"`, `"from":"elsewhere"`, 1), confirm: true, code: "不允许直接完成"},
		{name: "missing item version", id: "task", doc: strings.Replace(doneSnapshot, `"version":3`, `"version":null`, 1), code: "version"},
		{name: "unknown ID", id: "missing", doc: doneSnapshot, code: "不存在"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, _, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "PATCH" {
					t.Fatal("unsafe patch")
				}
				if strings.HasSuffix(r.URL.Path, "/preview") {
					_, _ = io.WriteString(w, tc.preview)
				} else {
					_, _ = io.WriteString(w, tc.doc)
				}
			})
			args := []string{"items", "done", tc.id}
			if tc.confirm {
				args = append(args, "--confirm")
			}
			if Execute(args, opts) == 0 || !strings.Contains(errOut.String(), tc.code) {
				t.Fatal(errOut)
			}
		})
	}
}

func TestDoneConflictNeverRetries(t *testing.T) {
	for _, code := range []string{"VERSION_CONFLICT", "CONFIRMATION_STALE", "INVALID_WORKFLOW_TRANSITION"} {
		t.Run(code, func(t *testing.T) {
			patches := 0
			opts, _, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "PATCH" {
					patches++
					w.WriteHeader(http.StatusConflict)
					fmt.Fprintf(w, `{"code":%q,"message":"conflict"}`, code)
				} else {
					_, _ = io.WriteString(w, doneSnapshot)
				}
			})
			if Execute([]string{"items", "done", "task", "--audience", "agent"}, opts) == 0 || patches != 1 || !strings.Contains(errOut.String(), code) {
				t.Fatal(patches, errOut)
			}
		})
	}
}

func TestCompletionTargetContract(t *testing.T) {
	for _, tc := range []struct {
		name, flow, kind, id string
		fail                 bool
	}{
		{name: "state edge", flow: `{"states":[{"id":"start","status":"TODO"},{"id":"end","status":"COMPLETED"}],"transitions":[{"from":"start","to":"end"}]}`, kind: "STATE", id: "end"},
		{name: "legacy macro", flow: `{"states":[{"id":"start","status":"TODO"},{"id":"end","status":"COMPLETED"}],"transitions":[{"from":"start","to":"COMPLETED","targetType":"MACRO"}]}`, kind: "MACRO", id: "COMPLETED"},
		{name: "custom macro", flow: `{"macroStates":[{"id":"finish","status":"COMPLETED"}],"states":[{"id":"start","status":"TODO"},{"id":"end","status":"COMPLETED"}],"transitions":[{"from":"start","to":"finish","targetType":"MACRO"}]}`, kind: "MACRO", id: "finish"},
		{name: "ambiguous", flow: `{"states":[{"id":"start","status":"TODO"},{"id":"a","status":"COMPLETED"},{"id":"b","status":"COMPLETED"}],"transitions":[{"from":"start","to":"a"},{"from":"start","to":"b"}]}`, fail: true},
		{name: "no skipping", flow: `{"states":[{"id":"start","status":"TODO"},{"id":"review","status":"IN_PROGRESS"},{"id":"end","status":"COMPLETED"}],"transitions":[{"from":"start","to":"review"},{"from":"review","to":"end"}]}`, fail: true},
		{name: "duplicate landing", flow: `{"states":[{"id":"start","status":"TODO"},{"id":"end","status":"COMPLETED"}],"transitions":[{"from":"start","to":"end"},{"from":"start","to":"COMPLETED","targetType":"MACRO"}]}`, kind: "STATE", id: "end"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var flow map[string]any
			if err := json.Unmarshal([]byte(tc.flow), &flow); err != nil {
				t.Fatal(err)
			}
			flow["id"] = "flow"
			state := map[string]any{"workflows": []any{flow}}
			item := map[string]any{"workflowId": "flow", "workflowStateId": "start"}
			got, err := completionTarget(state, item)
			if tc.fail {
				if err == nil {
					t.Fatal(got)
				}
				return
			}
			if err != nil || got["type"] != tc.kind || got["id"] != tc.id {
				t.Fatal(got, err)
			}
		})
	}
}
