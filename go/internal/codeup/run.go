package codeup

import (
	"errors"
	"fmt"
	"io"
	"os"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
	"git.internal.linran.top/linran/devkit-cli/internal/doctor"
)

type runEnv struct {
	Cwd    string
	Client *Client
}

// Dispatch 按已解析的命令执行。渲染和 audience 绑在一起，子命令不再各自决定输出格式。
func Dispatch(cmd Cmd, rt Runtime, env runEnv) (string, error) {
	cli := env.Client
	if cli == nil {
		return "", fail(1, "codeup client is required")
	}
	switch cmd.Kind {
	case kindInit:
		return runInit(rt, cli)
	case kindRepos:
		return runRepos(cmd, rt, cli)
	case kindPush:
		return runPush(cmd, rt, env.Cwd)
	case kindCrList:
		return runCrList(cmd, rt, cli, env.Cwd)
	case kindCrGet:
		return runCrGet(cmd, rt, cli, env.Cwd)
	case kindCrCreate:
		return runCrCreate(cmd, rt, cli, env.Cwd)
	case kindCrMerge:
		return runCrMerge(cmd, rt, cli, env.Cwd)
	case kindHookList:
		return runHooks(cmd, rt, cli, env.Cwd)
	default:
		return "", failf(1, "unhandled command: %s", cmd.Kind)
	}
}

func runInit(rt Runtime, cli *Client) (string, error) {
	repos, err := cli.ListAllRepos()
	if err != nil {
		return "", err
	}
	idx, err := SaveIndex(rt, repos)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(idx.Repos))
	for _, repo := range idx.Repos {
		names = append(names, repo.PathNs)
	}
	if names == nil {
		names = []string{}
	}
	out := initJSON{
		Profile:      idx.Profile,
		Count:        len(idx.Repos),
		UpdatedAt:    idx.UpdatedAt,
		Repositories: names,
	}
	return pickBody(rt.Audience, fmt.Sprintf("indexed %d repositories\n", len(idx.Repos)), out)
}

func runRepos(cmd Cmd, rt Runtime, cli *Client) (string, error) {
	repos, err := cli.ListRepos(repoQuery{Search: cmd.Search, Page: cmd.Page, PerPage: cmd.PerPage})
	if err != nil {
		return "", err
	}
	if repos == nil {
		repos = []Repo{}
	}
	out := reposJSON{Repositories: repos, Count: len(repos)}
	return pickBody(rt.Audience, reposText(repos), out)
}

func runPush(cmd Cmd, rt Runtime, cwd string) (string, error) {
	branch := cmd.Branch
	if !cmd.HasBranch {
		got, err := CurrentBranch(cwd)
		if err != nil {
			return "", err
		}
		branch = got
	}
	if err := PushBranch(cmd.Remote, branch, cwd); err != nil {
		return "", err
	}
	text := fmt.Sprintf("pushed %s to %s\n", branch, cmd.Remote)
	out := pushJSON{Remote: cmd.Remote, Branch: branch, Pushed: true}
	return pickBody(rt.Audience, text, out)
}

func runCrList(cmd Cmd, rt Runtime, cli *Client, cwd string) (string, error) {
	repo, err := ResolveRepo(repoPick{OrgID: rt.OrgID, Profile: rt.Profile.Name, Flag: cmd.Repo, Cwd: cwd})
	if err != nil {
		return "", err
	}
	list, err := cli.ListCrs(crQuery{
		Repo:    repo,
		State:   cmd.State,
		Source:  cmd.Source,
		Target:  cmd.Target,
		Search:  cmd.Search,
		Page:    cmd.Page,
		PerPage: cmd.PerPage,
	})
	if err != nil {
		return "", err
	}
	if list == nil {
		list = []ChangeRequest{}
	}
	out := crListJSON{Repo: repo, ChangeRequests: list, Count: len(list)}
	return pickBody(rt.Audience, crListText(repo, list), out)
}

func runCrGet(cmd Cmd, rt Runtime, cli *Client, cwd string) (string, error) {
	repo, err := ResolveRepo(repoPick{OrgID: rt.OrgID, Profile: rt.Profile.Name, Flag: cmd.Repo, Cwd: cwd})
	if err != nil {
		return "", err
	}
	cr, err := cli.GetCr(repo, cmd.LocalID)
	if err != nil {
		return "", err
	}
	return pickBody(rt.Audience, crViewText(repo, cr), flatCR(repo, cr))
}

func runCrCreate(cmd Cmd, rt Runtime, cli *Client, cwd string) (string, error) {
	repo, err := ResolveRepo(repoPick{OrgID: rt.OrgID, Profile: rt.Profile.Name, Flag: cmd.Repo, Cwd: cwd})
	if err != nil {
		return "", err
	}
	target := ""
	if cmd.Target != nil {
		target = *cmd.Target
	} else {
		got, err := cli.GetRepo(repo)
		if err != nil {
			return "", err
		}
		target = got.DefBranch
	}
	if target == "" {
		return "", failf(1, "repository %s has empty defaultBranch; pass --target", repo)
	}
	body, err := readBody(cmd.Body, cmd.BodyFile)
	if err != nil {
		return "", err
	}
	source := ""
	if cmd.Source != nil {
		source = *cmd.Source
	}
	cr, err := cli.CreateCr(crCreate{
		Repo:   repo,
		Source: source,
		Target: target,
		Title:  cmd.Title,
		Body:   body,
	})
	if err != nil {
		return "", err
	}
	text := fmt.Sprintf("created CR #%s in %s: %s\n", cr.LocalID, repo, cr.Title)
	if cr.CrURL != "" {
		text += cr.CrURL + "\n"
	}
	return pickBody(rt.Audience, text, flatCR(repo, cr))
}

func runCrMerge(cmd Cmd, rt Runtime, cli *Client, cwd string) (string, error) {
	repo, err := ResolveRepo(repoPick{OrgID: rt.OrgID, Profile: rt.Profile.Name, Flag: cmd.Repo, Cwd: cwd})
	if err != nil {
		return "", err
	}
	cr, err := cli.MergeCr(repo, cmd.LocalID, crMerge{Type: cmd.MergeType, Message: cmd.Message})
	if err != nil {
		return "", err
	}
	text := fmt.Sprintf("merged CR #%s in %s (%s): %s\n", cr.LocalID, repo, cmd.MergeType, cr.Title)
	if cr.CrURL != "" {
		text += cr.CrURL + "\n"
	}
	return pickBody(rt.Audience, text, mergeFlat(repo, cmd.MergeType, cr))
}

func runHooks(cmd Cmd, rt Runtime, cli *Client, cwd string) (string, error) {
	repo, err := ResolveRepo(repoPick{OrgID: rt.OrgID, Profile: rt.Profile.Name, Flag: cmd.Repo, Cwd: cwd})
	if err != nil {
		return "", err
	}
	hooks, err := cli.ListHooks(repo, cmd.Page, cmd.PerPage)
	if err != nil {
		return "", err
	}
	masked := MaskHooks(hooks, cmd.ShowSecrets)
	if masked == nil {
		masked = []Webhook{}
	}
	out := hooksJSON{Repo: repo, Webhooks: masked, Count: len(masked)}
	return pickBody(rt.Audience, webhookText(masked), out)
}

func flatCR(repo string, cr ChangeRequest) crFlatJSON {
	return crFlatJSON{
		Repo:        repo,
		LocalID:     cr.LocalID,
		Title:       cr.Title,
		State:       cr.State,
		SrcBranch:   cr.SrcBranch,
		TgtBranch:   cr.TgtBranch,
		Author:      cr.Author,
		CreatedAt:   cr.CreatedAt,
		UpdatedAt:   cr.UpdatedAt,
		WebURL:      cr.WebURL,
		CrURL:       cr.CrURL,
		Description: cr.Description,
	}
}

func mergeFlat(repo, kind string, cr ChangeRequest) crMergeJSON {
	flat := flatCR(repo, cr)
	return crMergeJSON{
		Repo:        repo,
		MergeType:   kind,
		LocalID:     flat.LocalID,
		Title:       flat.Title,
		State:       flat.State,
		SrcBranch:   flat.SrcBranch,
		TgtBranch:   flat.TgtBranch,
		Author:      flat.Author,
		CreatedAt:   flat.CreatedAt,
		UpdatedAt:   flat.UpdatedAt,
		WebURL:      flat.WebURL,
		CrURL:       flat.CrURL,
		Description: flat.Description,
	}
}

func readBody(body, bodyFile *string) (*string, error) {
	if body != nil && bodyFile != nil {
		return nil, fail(1, "use either --body or --body-file")
	}
	if bodyFile == nil {
		return body, nil
	}
	buf, err := os.ReadFile(*bodyFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, failf(1, "body file not found: %s", *bodyFile)
		}
		return nil, err
	}
	text := string(buf)
	return &text, nil
}

// Doctor 只检查本机配置能不能解析。不联网，缺文件或字段不合法就非 0。
func Doctor(w io.Writer, cfgPath string) error {
	var check error
	resolved, err := resolveCfgPath(cfgPath)
	if err != nil {
		check = err
	} else {
		check = checkCfg(resolved)
	}
	err = doctor.Write(w, []doctor.Item{{Name: "config", Err: check}})
	if err == nil {
		return nil
	}
	if err.Error() == "doctor failed" {
		return &ExitErr{Code: 1, Silent: true}
	}
	return err
}

func checkCfg(path string) error {
	buf, err := cfgfile.Read(path, cfgExample)
	if err != nil {
		return err
	}
	cfg, err := ParseFileCfg(string(buf), path)
	if err != nil {
		return err
	}
	_, err = PickProfile(cfg, "")
	return err
}
