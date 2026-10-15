package td

import (
	"encoding/json"
	"fmt"
	"time"
)

// login 走页面的 loginForToken. TE 4.4 没有设备码登录.
func login(host, loginName, password string) (string, error) {
	origin, err := originOf(host)
	if err != nil {
		return "", err
	}
	enc, err := encryptPwd(password)
	if err != nil {
		return "", err
	}
	form := formBody([][2]string{
		{"loginName", loginName},
		{"enPassword", enc},
		{"locale", "zh_CN"},
	})
	data, err := apiSend(httpSend{
		URL:    origin + "/v1/oauth/loginForToken",
		Method: "POST",
		Form:   form,
		FormOK: true,
	})
	if err != nil {
		return "", err
	}
	return readToken(data)
}

// listProjects 读登录校验里的 projectInfoList. TE 4.4 的 /v1/ta/project/list 会 500.
func listProjects(session Session) (json.RawMessage, error) {
	origin, err := originOf(session.Host)
	if err != nil {
		return nil, err
	}
	data, err := apiSend(httpSend{
		URL:    origin + "/v1/oauth/check",
		Method: "POST",
		Token:  session.Token,
		Form:   "",
		FormOK: true,
	})
	if err != nil {
		return nil, err
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("project list missing")
	}
	list, ok := obj["projectInfoList"]
	if !ok || len(list) == 0 || list[0] != '[' {
		return nil, fmt.Errorf("project list missing")
	}
	return list, nil
}

func listEvents(session Session, projectID string, now time.Time) (json.RawMessage, error) {
	root, err := taRoot(session.Host, session.BFF)
	if err != nil {
		return nil, err
	}
	u, err := withQuery(root+"/event/catalog/listEvent", [][2]string{{"projectId", projectID}}, true, now)
	if err != nil {
		return nil, err
	}
	return apiSend(httpSend{URL: u, Method: "GET", Token: session.Token})
}

func listProps(session Session, projectID string, event *string, now time.Time) (json.RawMessage, error) {
	root, err := taRoot(session.Host, session.BFF)
	if err != nil {
		return nil, err
	}
	if event == nil {
		u, err := withQuery(root+"/user/loadUserProps", [][2]string{{"projectId", projectID}}, true, now)
		if err != nil {
			return nil, err
		}
		return apiSend(httpSend{URL: u, Method: "GET", Token: session.Token})
	}
	u, err := withQuery(root+"/event/model/meta/loadFiltProps", [][2]string{{"projectId", projectID}}, false, now)
	if err != nil {
		return nil, err
	}
	body, err := marshalCompact(map[string]any{
		"data": map[string]any{
			"commonHeader": map[string]any{"projectId": json.RawMessage(jsNumber(projectID))},
			"eventModel":   0,
			"events": []any{map[string]any{
				"eventName": *event,
				"eventDesc": *event,
				"eventType": "event",
			}},
		},
	})
	if err != nil {
		return nil, err
	}
	return apiSend(httpSend{URL: u, Method: "POST", Token: session.Token, JSON: body})
}

func listReports(session Session, projectID string, now time.Time) (json.RawMessage, error) {
	root, err := taRoot(session.Host, session.BFF)
	if err != nil {
		return nil, err
	}
	u, err := withQuery(root+"/event/reportlist", [][2]string{{"projectId", projectID}}, true, now)
	if err != nil {
		return nil, err
	}
	return apiSend(httpSend{URL: u, Method: "GET", Token: session.Token})
}

func getReport(session Session, projectID, reportID string, now time.Time) (json.RawMessage, error) {
	root, err := taRoot(session.Host, session.BFF)
	if err != nil {
		return nil, err
	}
	u, err := withQuery(root+"/event/reportsearch", [][2]string{
		{"projectId", projectID},
		{"reportId", reportID},
		{"searchSource", "model_search"},
	}, true, now)
	if err != nil {
		return nil, err
	}
	return apiSend(httpSend{URL: u, Method: "GET", Token: session.Token})
}

func readToken(data json.RawMessage) (string, error) {
	v, err := decodeJSON(data)
	if err != nil {
		return "", err
	}
	obj, ok := asMap(v)
	if !ok {
		return "", fmt.Errorf("login response has no tokenInfo")
	}
	info, ok := asMap(obj["tokenInfo"])
	if !ok {
		return "", fmt.Errorf("login response has no tokenInfo")
	}
	token := asText(info["accessToken"])
	if token == "" {
		return "", fmt.Errorf("login response has no accessToken")
	}
	return token, nil
}
