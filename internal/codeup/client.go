package codeup

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// Client 收拢云效 OpenAPI。token、组织和仓库编码散开会写出第二套客户端。
type Client struct {
	RT   Runtime
	Base string
	HTTP *http.Client
}

type repoQuery struct {
	Search  string
	Page    int
	PerPage int
}

type crQuery struct {
	Repo    string
	State   string
	Source  *string
	Target  *string
	Search  string
	Page    int
	PerPage int
}

type crCreate struct {
	Repo   string
	Source string
	Target string
	Title  string
	Body   *string
}

type crMerge struct {
	Type    string
	Message *string
}

type repoPage struct {
	Page       int
	TotalPages int
	Total      int
	Repos      []Repo
}

type pair struct {
	key   string
	value string
}

func (c *Client) base() string {
	if c.Base != "" {
		return strings.TrimRight(c.Base, "/")
	}
	if c.RT.APIBase != "" {
		return strings.TrimRight(c.RT.APIBase, "/")
	}
	return APIBase
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

// ListRepos 拉一页仓库。搜索和分页都原样进 query，不在客户端再滤一遍。
func (c *Client) ListRepos(q repoQuery) ([]Repo, error) {
	page, err := c.listRepoPage(q)
	if err != nil {
		return nil, err
	}
	return page.Repos, nil
}

// ListAllRepos 给 init 拉全量。页数不齐就失败，避免留下半截索引。
func (c *Client) ListAllRepos() ([]Repo, error) {
	const perPage = 100
	first, err := c.listRepoPage(repoQuery{Page: 1, PerPage: perPage})
	if err != nil {
		return nil, err
	}
	if first.TotalPages > 200 {
		return nil, fail(3, "repository list exceeded 200 pages")
	}
	if first.TotalPages <= 1 {
		return first.Repos, nil
	}
	rest := make([][]Repo, first.TotalPages-1)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failed error
	for page := 2; page <= first.TotalPages; page++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			got, err := c.listRepoPage(repoQuery{Page: page, PerPage: perPage})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if failed == nil {
					failed = err
				}
				return
			}
			rest[page-2] = got.Repos
		}(page)
	}
	wg.Wait()
	if failed != nil {
		return nil, failed
	}
	out := append([]Repo{}, first.Repos...)
	for _, batch := range rest {
		out = append(out, batch...)
	}
	return out, nil
}

func (c *Client) listRepoPage(q repoQuery) (repoPage, error) {
	raw, err := c.requestRaw(http.MethodGet, c.orgPath("/repositories"), []pair{
		{key: "search", value: q.Search},
		{key: "page", value: strconv.Itoa(q.Page)},
		{key: "perPage", value: strconv.Itoa(q.PerPage)},
		{key: "orderBy", value: "name"},
		{key: "sort", value: "asc"},
	}, nil)
	if err != nil {
		return repoPage{}, err
	}
	meta, err := parsePageMeta(raw.header)
	if err != nil {
		return repoPage{}, err
	}
	if meta.page != q.Page {
		return repoPage{}, failf(3, "repository page mismatch: want %d got %d", q.Page, meta.page)
	}
	items, err := asArray(raw.json, "repository list")
	if err != nil {
		return repoPage{}, err
	}
	repos := make([]Repo, 0, len(items))
	for _, item := range items {
		repo, err := toRepo(item)
		if err != nil {
			return repoPage{}, err
		}
		repos = append(repos, repo)
	}
	return repoPage{Page: meta.page, TotalPages: meta.totalPages, Total: meta.total, Repos: repos}, nil
}

// GetRepo 读单个仓库。创建变更前要拿默认分支和数字 ID。
func (c *Client) GetRepo(repo string) (Repo, error) {
	path, err := c.repoPath(repo, "")
	if err != nil {
		return Repo{}, err
	}
	raw, err := c.request(http.MethodGet, path, nil, nil)
	if err != nil {
		return Repo{}, err
	}
	return toRepo(raw)
}

// ListCrs 按仓库列变更。source/target 接口没有查询参数，只能拿回当前页再滤。
func (c *Client) ListCrs(q crQuery) ([]ChangeRequest, error) {
	repoID, err := c.numericID(q.Repo)
	if err != nil {
		return nil, err
	}
	state := q.State
	if state == "all" {
		state = ""
	}
	raw, err := c.request(http.MethodGet, c.orgPath("/changeRequests"), []pair{
		{key: "projectIds", value: repoID},
		{key: "state", value: state},
		{key: "search", value: q.Search},
		{key: "page", value: strconv.Itoa(q.Page)},
		{key: "perPage", value: strconv.Itoa(q.PerPage)},
		{key: "orderBy", value: "updated_at"},
		{key: "sort", value: "desc"},
	}, nil)
	if err != nil {
		return nil, err
	}
	items, err := asArray(raw, "change request list")
	if err != nil {
		return nil, err
	}
	list := make([]ChangeRequest, 0, len(items))
	for _, item := range items {
		cr, err := toCr(item)
		if err != nil {
			return nil, err
		}
		if q.Source != nil && cr.SrcBranch != *q.Source {
			continue
		}
		if q.Target != nil && cr.TgtBranch != *q.Target {
			continue
		}
		list = append(list, cr)
	}
	return list, nil
}

// GetCr 读一条变更。合并后再读一次，是因为合并响应用的是另一套状态字段。
func (c *Client) GetCr(repo, localID string) (ChangeRequest, error) {
	path, err := c.repoPath(repo, "/changeRequests/"+localID)
	if err != nil {
		return ChangeRequest{}, err
	}
	raw, err := c.request(http.MethodGet, path, nil, nil)
	if err != nil {
		return ChangeRequest{}, err
	}
	return toCr(raw)
}

// CreateCr 创建变更。远端没有源分支就直接失败，不把 400 留给用户猜。
func (c *Client) CreateCr(q crCreate) (ChangeRequest, error) {
	repoID, err := c.numericID(q.Repo)
	if err != nil {
		return ChangeRequest{}, err
	}
	projectID, err := toProjectID(repoID)
	if err != nil {
		return ChangeRequest{}, err
	}
	if err := c.requireRemoteBranch(q.Repo, q.Source); err != nil {
		return ChangeRequest{}, err
	}
	body := crCreateBody(projectID, q)
	path, err := c.repoPath(q.Repo, "/changeRequests")
	if err != nil {
		return ChangeRequest{}, err
	}
	raw, err := c.request(http.MethodPost, path, nil, body)
	if err != nil {
		return ChangeRequest{}, err
	}
	return toCr(raw)
}

// MergeCr 按调用方给的 mergeType 合并，再读回统一字段。
func (c *Client) MergeCr(repo, localID string, q crMerge) (ChangeRequest, error) {
	path, err := c.repoPath(repo, "/changeRequests/"+localID+"/merge")
	if err != nil {
		return ChangeRequest{}, err
	}
	if _, err := c.request(http.MethodPost, path, nil, crMergeBody(q)); err != nil {
		return ChangeRequest{}, err
	}
	return c.GetCr(repo, localID)
}

// ListHooks 列 webhook。路径用数字仓库 ID，和变更列表的编码方式不同。
func (c *Client) ListHooks(repo string, page, perPage int) ([]Webhook, error) {
	repoID, err := c.numericID(repo)
	if err != nil {
		return nil, err
	}
	path := c.orgPath("/repositories/" + encURI(repoID) + "/webhooks")
	raw, err := c.request(http.MethodGet, path, []pair{
		{key: "page", value: strconv.Itoa(page)},
		{key: "perPage", value: strconv.Itoa(perPage)},
	}, nil)
	if err != nil {
		return nil, err
	}
	items, err := asArray(raw, "webhook list")
	if err != nil {
		return nil, err
	}
	hooks := make([]Webhook, 0, len(items))
	for _, item := range items {
		hook, err := toHook(item)
		if err != nil {
			return nil, err
		}
		hooks = append(hooks, hook)
	}
	return hooks, nil
}

func (c *Client) requireRemoteBranch(repo, branch string) error {
	path, err := c.repoPath(repo, "/branches/"+encURI(branch))
	if err != nil {
		return err
	}
	_, err = c.request(http.MethodGet, path, nil, nil)
	if err == nil {
		return nil
	}
	var exit *ExitErr
	if errors.As(err, &exit) && exit.Code == 2 {
		return failf(1, "source branch not on remote: %s; push first", branch)
	}
	return err
}

func (c *Client) numericID(repo string) (string, error) {
	got, err := c.GetRepo(repo)
	if err != nil {
		return "", err
	}
	if got.ID == "" {
		return "", failf(3, "could not resolve numeric id for %s", repo)
	}
	return got.ID, nil
}

func (c *Client) orgPath(tail string) string {
	return "/oapi/v1/codeup/organizations/" + c.RT.OrgID + tail
}

func (c *Client) repoPath(repo, tail string) (string, error) {
	enc, err := EncodeRepoID(repo, c.RT.OrgID)
	if err != nil {
		return "", err
	}
	return c.orgPath("/repositories/") + enc + tail, nil
}

func (c *Client) redact(text string) string {
	if c.RT.OrgID == "" {
		return text
	}
	return strings.ReplaceAll(text, c.RT.OrgID, "<organizationId>")
}

type rawResp struct {
	json   any
	header http.Header
}

func (c *Client) request(method, path string, query []pair, body any) (any, error) {
	raw, err := c.requestRaw(method, path, query, body)
	if err != nil {
		return nil, err
	}
	return raw.json, nil
}

func (c *Client) requestRaw(method, path string, query []pair, body any) (rawResp, error) {
	rawURL := c.base() + path
	if q := encodeQuery(query); q != "" {
		rawURL += "?" + q
	}
	var payload io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return rawResp{}, err
		}
		payload = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, rawURL, payload)
	if err != nil {
		return rawResp{}, fail(1, err.Error())
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-yunxiao-token", c.RT.Token)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return rawResp{}, fail(1, err.Error())
	}
	defer resp.Body.Close()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return rawResp{}, fail(1, err.Error())
	}
	loc := requestPath(req)
	text := string(buf)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := 3
		if resp.StatusCode == http.StatusNotFound {
			code = 2
		}
		msg := "Codeup API " + strconv.Itoa(resp.StatusCode) + " " + http.StatusText(resp.StatusCode) + ": " + c.redact(loc)
		if text != "" {
			msg += ": " + c.redact(clipBody(text))
		}
		return rawResp{}, fail(code, msg)
	}
	if text == "" {
		return rawResp{}, failf(3, "Codeup API empty body: %s", c.redact(loc))
	}
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.UseNumber()
	var parsed any
	if err := dec.Decode(&parsed); err != nil {
		return rawResp{}, failf(3, "Codeup API invalid JSON: %s", c.redact(loc))
	}
	return rawResp{json: parsed, header: resp.Header}, nil
}

func requestPath(req *http.Request) string {
	if req.URL == nil {
		return ""
	}
	if esc := req.URL.EscapedPath(); esc != "" {
		return esc
	}
	return req.URL.Path
}

func encodeQuery(query []pair) string {
	var b strings.Builder
	first := true
	for _, item := range query {
		if item.value == "" {
			continue
		}
		if !first {
			b.WriteByte('&')
		}
		first = false
		b.WriteString(url.QueryEscape(item.key))
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(item.value))
	}
	return b.String()
}

type createBody struct {
	Title           string  `json:"title"`
	SourceBranch    string  `json:"sourceBranch"`
	TargetBranch    string  `json:"targetBranch"`
	SourceProjectID int     `json:"sourceProjectId"`
	TargetProjectID int     `json:"targetProjectId"`
	Description     *string `json:"description,omitempty"`
}

type mergeBody struct {
	MergeType    string  `json:"mergeType"`
	MergeMessage *string `json:"mergeMessage,omitempty"`
}

func crCreateBody(projectID int, q crCreate) createBody {
	return createBody{
		Title:           q.Title,
		SourceBranch:    q.Source,
		TargetBranch:    q.Target,
		SourceProjectID: projectID,
		TargetProjectID: projectID,
		Description:     q.Body,
	}
}

func crMergeBody(q crMerge) mergeBody {
	return mergeBody{MergeType: q.Type, MergeMessage: q.Message}
}

func toProjectID(repoID string) (int, error) {
	if !digitRE.MatchString(repoID) {
		return 0, failf(3, "invalid repository project id: %s", repoID)
	}
	n, err := strconv.Atoi(repoID)
	if err != nil {
		return 0, failf(3, "invalid repository project id: %s", repoID)
	}
	return n, nil
}

func clipBody(text string) string {
	if len(text) <= 300 {
		return text
	}
	return text[:300]
}

type pageMeta struct {
	page       int
	totalPages int
	total      int
}

func parsePageMeta(header http.Header) (pageMeta, error) {
	page, err := headerInt(header, "x-page")
	if err != nil {
		return pageMeta{}, err
	}
	totalPages, err := headerInt(header, "x-total-pages")
	if err != nil {
		return pageMeta{}, err
	}
	total, err := headerInt(header, "x-total")
	if err != nil {
		return pageMeta{}, err
	}
	return pageMeta{page: page, totalPages: totalPages, total: total}, nil
}

func headerInt(header http.Header, name string) (int, error) {
	raw := strings.TrimSpace(header.Get(name))
	if raw == "" {
		return 0, failf(3, "missing %s header", name)
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, failf(3, "invalid %s header: %s", name, header.Get(name))
	}
	return value, nil
}

func asArray(raw any, label string) ([]any, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, failf(3, "%s: expected array", label)
	}
	return items, nil
}

func asRec(raw any, label string) (map[string]any, error) {
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, failf(3, "%s: expected object", label)
	}
	return obj, nil
}

func asID(value any) (string, bool) {
	switch id := value.(type) {
	case string:
		return id, true
	case json.Number:
		return id.String(), true
	default:
		return "", false
	}
}

func asBool(value any) (bool, error) {
	flag, ok := value.(bool)
	if !ok {
		return false, fail(3, "expected boolean field")
	}
	return flag, nil
}

func toRepo(raw any) (Repo, error) {
	rec, err := asRec(raw, "repository")
	if err != nil {
		return Repo{}, err
	}
	id, ok := asID(rec["id"])
	if !ok {
		return Repo{}, fail(3, "repository missing id")
	}
	pathNs := readStr(rec["pathWithNamespace"])
	path := readStr(rec["path"])
	if pathNs == "" {
		pathNs = path
	}
	return Repo{
		ID:        id,
		Name:      readStr(rec["name"]),
		Path:      path,
		PathNs:    pathNs,
		WebURL:    readStr(rec["webUrl"]),
		DefBranch: readStr(rec["defaultBranch"]),
	}, nil
}

func toCr(raw any) (ChangeRequest, error) {
	rec, err := asRec(raw, "change request")
	if err != nil {
		return ChangeRequest{}, err
	}
	localID, ok := asID(rec["localId"])
	if !ok {
		return ChangeRequest{}, fail(3, "change request missing localId")
	}
	webURL := readStr(rec["webUrl"])
	crURL := ""
	if webURL != "" {
		crURL = webURL + "/change/" + localID
	}
	return ChangeRequest{
		LocalID:     localID,
		Title:       readStr(rec["title"]),
		State:       readStr(rec["state"]),
		SrcBranch:   readStr(rec["sourceBranch"]),
		TgtBranch:   readStr(rec["targetBranch"]),
		Author:      readStr(rec["authorUserName"]),
		CreatedAt:   readStr(rec["createdAt"]),
		UpdatedAt:   readStr(rec["updatedAt"]),
		WebURL:      webURL,
		CrURL:       crURL,
		Description: readStr(rec["description"]),
	}, nil
}

func toHook(raw any) (Webhook, error) {
	rec, err := asRec(raw, "webhook")
	if err != nil {
		return Webhook{}, err
	}
	id, ok := asID(rec["id"])
	if !ok {
		return Webhook{}, fail(3, "webhook missing id")
	}
	push, err := asBool(rec["pushEvents"])
	if err != nil {
		return Webhook{}, err
	}
	mr, err := asBool(rec["mergeRequestEvents"])
	if err != nil {
		return Webhook{}, err
	}
	tag, err := asBool(rec["tagPushEvents"])
	if err != nil {
		return Webhook{}, err
	}
	note, err := asBool(rec["noteEvents"])
	if err != nil {
		return Webhook{}, err
	}
	return Webhook{
		ID:                  id,
		URL:                 readStr(rec["url"]),
		SecretToken:         readStr(rec["token"]),
		PushEvents:          push,
		MergeRequestsEvents: mr,
		TagPushEvents:       tag,
		NoteEvents:          note,
		CreatedAt:           readStr(rec["createdAt"]),
		UpdatedAt:           readStr(rec["updatedAt"]),
	}, nil
}
