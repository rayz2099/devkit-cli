package nacos

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	defaultCfgGroup    = "COMMON"
	defaultNamingGroup = "DEFAULT_GROUP"
	defaultCluster     = "DEFAULT"
)

func (a *app) runtime(g globals) (runtime, error) {
	file, err := loadCfg(g.configValue())
	if err != nil {
		return runtime{}, err
	}
	return resolveRuntime(g, file)
}

func (a *app) write(text string) error {
	_, err := io.WriteString(a.out, text)
	return err
}

func (a *app) runGet(args []string) error {
	g, rest, err := splitGlobal(args)
	if err != nil {
		return err
	}
	if hasHelp(args) {
		return a.write(helpText("config"))
	}
	cmd := takeCmd(rest)
	dataID := firstNonEmpty(cmd.get("--data-id"), at(cmd.positionals, 0))
	group := firstNonEmpty(cmd.get("--group"), at(cmd.positionals, 1), defaultCfgGroup)
	if dataID == "" {
		return errors.New("data-id is required")
	}
	if group == "" {
		return errors.New("group is required")
	}
	rt, client, err := a.open(g)
	if err != nil {
		return err
	}
	content, err := client.getConfig(dataID, group)
	if err != nil {
		return err
	}
	text, err := render(rt.output, content, gotView{DataID: dataID, Group: group, Content: content})
	if err != nil {
		return err
	}
	return a.write(text)
}

func (a *app) runPut(args []string) error {
	g, rest, err := splitGlobal(args)
	if err != nil {
		return err
	}
	if hasHelp(args) {
		return a.write(helpText("config"))
	}
	cmd := takeCmd(rest)
	dataID := strings.TrimSpace(cmd.get("--data-id"))
	group := strings.TrimSpace(cmd.get("--group"))
	if dataID == "" {
		return errors.New("data-id is required")
	}
	if group == "" {
		return errors.New("group is required")
	}
	if !cmd.has("--content") {
		return errors.New("content is required")
	}
	rt, client, err := a.open(g)
	if err != nil {
		return err
	}
	if err := client.putConfig(dataID, group, cmd.get("--content")); err != nil {
		return err
	}
	return a.writeOK(rt)
}

func (a *app) runDelete(args []string) error {
	g, rest, err := splitGlobal(args)
	if err != nil {
		return err
	}
	if hasHelp(args) {
		return a.write(helpText("config"))
	}
	cmd := takeCmd(rest)
	dataID := strings.TrimSpace(cmd.get("--data-id"))
	group := strings.TrimSpace(cmd.get("--group"))
	if dataID == "" {
		return errors.New("data-id is required")
	}
	if group == "" {
		return errors.New("group is required")
	}
	rt, client, err := a.open(g)
	if err != nil {
		return err
	}
	if err := client.deleteConfig(dataID, group); err != nil {
		return err
	}
	return a.writeOK(rt)
}

func (a *app) runList(args []string) error {
	g, rest, err := splitGlobal(args)
	if err != nil {
		return err
	}
	if hasHelp(args) {
		return a.write(helpText("config"))
	}
	cmd := takeCmd(rest)
	search := "blur"
	if cmd.has("--search") {
		search = cmd.get("--search")
	}
	if search != "accurate" && search != "blur" {
		return errors.New("search must be one of: accurate, blur")
	}
	pageNo, err := parseIntFlag(cmd.get("--page-no"), cmd.has("--page-no"), 1, "page-no")
	if err != nil {
		return err
	}
	pageSize, err := parseIntFlag(cmd.get("--page-size"), cmd.has("--page-size"), 10, "page-size")
	if err != nil {
		return err
	}
	rt, client, err := a.open(g)
	if err != nil {
		return err
	}
	page, err := client.listConfigs(listCfgIn{
		search:   search,
		dataID:   cmd.get("--data-id"),
		group:    cmd.get("--group"),
		pageNo:   pageNo,
		pageSize: pageSize,
	})
	if err != nil {
		return err
	}
	if rt.output == "json" {
		text, err := render("json", "", page.view())
		if err != nil {
			return err
		}
		return a.write(text)
	}
	rows := make([][]string, 0, len(page.items))
	for _, item := range page.items {
		rows = append(rows, []string{item.dataID, item.group})
	}
	summary := fmt.Sprintf("Total: %s  Page: %s/%s", numText(page.total), numText(page.page), numText(page.pages))
	text, err := render("text", renderTable(summary, []string{"DATA_ID", "GROUP"}, rows), nil)
	if err != nil {
		return err
	}
	return a.write(text)
}

func (a *app) runRegister(args []string) error {
	g, rest, err := splitGlobal(args)
	if err != nil {
		return err
	}
	if hasHelp(args) {
		return a.write(helpText("naming"))
	}
	cmd := takeCmd(rest)
	service, err := requireFlag(cmd.flags, "--service")
	if err != nil {
		return err
	}
	ip, err := requireFlag(cmd.flags, "--ip")
	if err != nil {
		return err
	}
	port, err := requirePort(cmd.get("--port"), cmd.has("--port"))
	if err != nil {
		return err
	}
	weight, err := parseFloatFlag(cmd.get("--weight"), cmd.has("--weight"), 1, "weight")
	if err != nil {
		return err
	}
	ephemeral, err := parseBoolFlag(cmd.get("--ephemeral"), cmd.has("--ephemeral"), true)
	if err != nil {
		return err
	}
	group := defaultNamingGroup
	if cmd.has("--group") {
		group = cmd.get("--group")
	}
	cluster := defaultCluster
	if cmd.has("--cluster") {
		cluster = cmd.get("--cluster")
	}
	rt, client, err := a.open(g)
	if err != nil {
		return err
	}
	in := regIn{
		service:   service,
		ip:        ip,
		port:      port,
		group:     group,
		cluster:   cluster,
		weight:    weight,
		ephemeral: ephemeral,
	}
	if err := client.registerInst(in); err != nil {
		return err
	}
	return a.writeOK(rt)
}

func (a *app) runDeregister(args []string) error {
	g, rest, err := splitGlobal(args)
	if err != nil {
		return err
	}
	if hasHelp(args) {
		return a.write(helpText("naming"))
	}
	cmd := takeCmd(rest)
	service, err := requireFlag(cmd.flags, "--service")
	if err != nil {
		return err
	}
	ip, err := requireFlag(cmd.flags, "--ip")
	if err != nil {
		return err
	}
	port, err := requirePort(cmd.get("--port"), cmd.has("--port"))
	if err != nil {
		return err
	}
	ephemeral, err := parseBoolFlag(cmd.get("--ephemeral"), cmd.has("--ephemeral"), true)
	if err != nil {
		return err
	}
	group := defaultNamingGroup
	if cmd.has("--group") {
		group = cmd.get("--group")
	}
	cluster := defaultCluster
	if cmd.has("--cluster") {
		cluster = cmd.get("--cluster")
	}
	rt, client, err := a.open(g)
	if err != nil {
		return err
	}
	in := deregIn{
		service:   service,
		ip:        ip,
		port:      port,
		group:     group,
		cluster:   cluster,
		ephemeral: ephemeral,
	}
	if err := client.deregisterInst(in); err != nil {
		return err
	}
	return a.writeOK(rt)
}

func (a *app) runInstances(args []string) error {
	g, rest, err := splitGlobal(args)
	if err != nil {
		return err
	}
	if hasHelp(args) {
		return a.write(helpText("naming"))
	}
	cmd := takeCmd(rest)
	service, err := requireFlag(cmd.flags, "--service")
	if err != nil {
		return err
	}
	healthy, err := parseBoolFlag(cmd.get("--healthy-only"), cmd.has("--healthy-only"), true)
	if err != nil {
		return err
	}
	group := defaultNamingGroup
	if cmd.has("--group") {
		group = cmd.get("--group")
	}
	rt, client, err := a.open(g)
	if err != nil {
		return err
	}
	insts, err := client.listInsts(instQuery{
		service:     service,
		group:       group,
		clusters:    splitClusters(cmd.get("--clusters")),
		healthyOnly: healthy,
	})
	if err != nil {
		return err
	}
	if rt.output == "json" {
		views := make([]instView, 0, len(insts))
		for _, item := range insts {
			views = append(views, item.view())
		}
		text, err := render("json", "", views)
		if err != nil {
			return err
		}
		return a.write(text)
	}
	lines := make([]string, 0, 1+len(insts))
	lines = append(lines, fmt.Sprintf("count: %d", len(insts)))
	for _, item := range insts {
		lines = append(lines, item.ip+":"+numText(item.port))
	}
	text, err := render("text", strings.Join(lines, "\n"), nil)
	if err != nil {
		return err
	}
	return a.write(text)
}

func (a *app) runFishHidden(args []string) error {
	g, rest, err := splitGlobal(args)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		return errors.New("unknown fish completion: ")
	}
	kind := rest[0]
	flags := takeCmd(rest[1:])
	prefix := flags.get("--prefix")
	switch kind {
	case "namespaces":
		file, err := loadCfg(g.configValue())
		if err != nil {
			return err
		}
		return a.write(completeLines(nsCands(file), prefix))
	case "config-data-ids":
		_, client, err := a.open(g)
		if err != nil {
			return err
		}
		items, err := client.listItemsCached()
		if err != nil {
			return err
		}
		return a.write(completeLines(uniqueDataIDs(items, prefix), prefix))
	case "config-groups":
		_, client, err := a.open(g)
		if err != nil {
			return err
		}
		items, err := client.listItemsCached()
		if err != nil {
			return err
		}
		groups := groupsByDataID(items, flags.get("--data-id"), prefix)
		return a.write(completeLines(groups, prefix))
	default:
		return fmt.Errorf("unknown fish completion: %s", kind)
	}
}

func (a *app) runComplete(args []string) error {
	tokens, current := splitToComplete(args)
	text, err := finishComplete(tokens, current)
	if err != nil {
		return a.write(":4\n")
	}
	return a.write(text)
}

func finishComplete(tokens []string, current string) (string, error) {
	file, err := loadCfg(cfgFromTokens(tokens))
	if err != nil {
		return "", err
	}
	values, err := completeLine(tokens, current, file)
	if err != nil {
		return "", err
	}
	return cobraComp(values), nil
}

func cfgFromTokens(tokens []string) string {
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token != "-c" && token != "--config" {
			continue
		}
		if i+1 >= len(tokens) {
			return ""
		}
		val := tokens[i+1]
		if val != "" && !strings.HasPrefix(val, "-") {
			return val
		}
	}
	return ""
}

func (a *app) open(g globals) (runtime, *Client, error) {
	rt, err := a.runtime(g)
	if err != nil {
		return runtime{}, nil, err
	}
	client, err := newClient(rt)
	if err != nil {
		return runtime{}, nil, err
	}
	return rt, client, nil
}

func (a *app) writeOK(rt runtime) error {
	text, err := render(rt.output, "ok", okView{Success: true})
	if err != nil {
		return err
	}
	return a.write(text)
}

func numText(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
