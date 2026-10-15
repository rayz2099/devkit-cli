package td

import (
	"encoding/json"
	"fmt"
	"net/url"
)

const (
	wsHeartbeat = "heartbeat"
	wsProgress  = "progress"
	wsDone      = "done"
	wsFailed    = "failed"
	wsIgnore    = "ignore"
)

// wsMsg 是服务端文本帧拆开后的一种. 进度、结果和心跳共用一条连接.
type wsMsg struct {
	Kind     string
	ID       string
	Progress json.Number
	Hint     string
	Result   json.RawMessage
	Message  string
}

type queryPayload struct {
	RequestID    string `json:"requestId"`
	ProjectID    int    `json:"projectId"`
	EventModel   int    `json:"eventModel"`
	QP           string `json:"qp"`
	SearchSource string `json:"searchSource"`
	QuerySource  string `json:"querySource"`
}

type killPayload struct {
	RequestID   string `json:"requestId"`
	QueryStatus string `json:"queryStatus"`
}

// buildFrame 把计算请求收成三元组. 前缀只用于请求号, 路由看 eventModel.
func buildFrame(body queryPayload) (string, error) {
	data, err := marshalCompact(body)
	if err != nil {
		return "", err
	}
	return `["data",` + string(data) + `,{"channel":"ta"}]`, nil
}

// killFrame 沿用页面的 KILL 帧. 超时后用它把这次查询停掉.
func killFrame(id string) (string, error) {
	data, err := marshalCompact(killPayload{RequestID: id, QueryStatus: "KILL"})
	if err != nil {
		return "", err
	}
	return `["data",` + string(data) + `]`, nil
}

// wsURL 把 accessToken 拼进 websocket 路径. bff 开关会改这条路径.
func wsURL(host, token string, bff bool) (string, error) {
	origin, err := originOf(host)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(origin)
	if err != nil {
		return "", err
	}
	proto := "ws"
	if u.Scheme == "https" {
		proto = "wss"
	}
	base := proto + "://" + u.Host
	enc := encodeURIComponent(token)
	if bff {
		return base + "/bff/v1/ta-websocket/query?token=" + enc, nil
	}
	return base + "/v1/ta-websocket/query/" + enc, nil
}

// parseWS 只认自己发出的 requestId. 别的计算帧一律忽略.
func parseWS(raw string) wsMsg {
	var parsed []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil || len(parsed) == 0 {
		return wsMsg{Kind: wsIgnore}
	}
	var kind string
	if err := json.Unmarshal(parsed[0], &kind); err != nil {
		return wsMsg{Kind: wsIgnore}
	}
	if kind == "heartbeat" {
		return wsMsg{Kind: wsHeartbeat}
	}
	if kind != "data" || len(parsed) < 2 {
		return wsMsg{Kind: wsIgnore}
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(parsed[1], &payload); err != nil {
		return wsMsg{Kind: wsIgnore}
	}
	id := ""
	if rawID, ok := payload["requestId"]; ok {
		var text string
		if err := json.Unmarshal(rawID, &text); err == nil {
			id = text
		}
	}
	if status, ok := payload["status"]; ok {
		var text string
		if err := json.Unmarshal(status, &text); err == nil && text == "failed" {
			message := rawText(payload["errorMsg"])
			if message == "" {
				message = "query failed"
			}
			return wsMsg{Kind: wsFailed, ID: id, Message: message}
		}
	}
	if result, ok := payload["result"]; ok && id != "" {
		return wsMsg{Kind: wsDone, ID: id, Result: result}
	}
	if prog, ok := payload["progress"]; ok {
		var n json.Number
		if err := json.Unmarshal(prog, &n); err == nil {
			return wsMsg{Kind: wsProgress, ID: id, Progress: n, Hint: rawText(payload["hintMsg"])}
		}
	}
	return wsMsg{Kind: wsIgnore}
}

func requestID(prefix string) (string, error) {
	id, err := newUUID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s@@%s", prefix, id), nil
}
