package ghimage

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	aliasRE   = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`)
	versionRE = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
)

// image 是台账里的一条。node 留着，是为了改状态时不动旁边的字段和注释。
type image struct {
	kind   string
	source string
	script string
	node   *yaml.Node
}

// store 是仓库根 images.yaml。用节点而不是结构体往返，才能保住注释和版本顺序。
type store struct {
	path   string
	doc    *yaml.Node
	images *yaml.Node
}

func loadStore(path string) (*store, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(buf)) == 0 {
		return nil, errors.New("images.yaml must contain an images map")
	}
	doc := &yaml.Node{}
	if err := yaml.Unmarshal(buf, doc); err != nil {
		return nil, fmt.Errorf("invalid images.yaml: %s", err.Error())
	}
	root, err := mappingRoot(doc)
	if err != nil {
		return nil, err
	}
	images := mapGet(root, "images")
	if images == nil || images.Kind != yaml.MappingNode {
		if images == nil || images.Kind == yaml.ScalarNode || images.Tag == "!!null" {
			return nil, errors.New("images.yaml must contain an images map")
		}
		return nil, errors.New("images.yaml images must be a map")
	}
	return &store{path: path, doc: doc, images: images}, nil
}

func mappingRoot(doc *yaml.Node) (*yaml.Node, error) {
	root := doc
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) != 1 {
			return nil, errors.New("images.yaml must contain an images map")
		}
		root = doc.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("images.yaml must contain an images map")
	}
	return root, nil
}

func (s *store) save() error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(s.doc); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	// 只截断原文件，不改权限。配置目录本来就不许动，台账同样不 chmod。
	file, err := os.OpenFile(s.path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	_, err = file.Write(buf.Bytes())
	cerr := file.Close()
	if err != nil {
		return err
	}
	return cerr
}

func (s *store) aliases() []string {
	return mapKeys(s.images)
}

func (s *store) versions(alias string) ([]string, error) {
	img, err := s.require(alias)
	if err != nil {
		return nil, err
	}
	node := mapGet(img.node, "versions")
	if node == nil {
		return []string{}, nil
	}
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("versions must be a map: %s", alias)
	}
	return mapKeys(node), nil
}

func (s *store) require(alias string) (image, error) {
	img, ok, err := s.lookup(alias)
	if err != nil {
		return image{}, err
	}
	if !ok {
		return image{}, fmt.Errorf("image alias not found: %s", alias)
	}
	return img, nil
}

func (s *store) lookup(alias string) (image, bool, error) {
	node := mapGet(s.images, alias)
	if node == nil {
		return image{}, false, nil
	}
	if node.Kind != yaml.MappingNode {
		return image{}, false, fmt.Errorf("invalid image record: %s", alias)
	}
	img := image{node: node}
	if text, ok := scalar(mapGet(node, "type")); ok {
		img.kind = text
	}
	if text, ok := scalar(mapGet(node, "source")); ok {
		img.source = text
	}
	if text, ok := scalar(mapGet(node, "script")); ok {
		img.script = text
	}
	return img, true, nil
}

func (s *store) addMirror(source, alias, version string) error {
	if err := validateAlias(alias); err != nil {
		return err
	}
	if err := validateVersion(version); err != nil {
		return err
	}
	cur, ok, err := s.lookup(alias)
	if err != nil {
		return err
	}
	if ok && (cur.kind != "mirror" || cur.source != source) {
		return fmt.Errorf("alias %s already maps to a different image source", alias)
	}
	if !ok {
		s.addImage(alias, mirrorNode(source))
	}
	return s.setStatus(alias, version, "init", true)
}

func (s *store) addDockerfile(script, alias, version string) error {
	if err := validateAlias(alias); err != nil {
		return err
	}
	if err := validateVersion(version); err != nil {
		return err
	}
	if badScript(script) {
		return errors.New("Dockerfile script must be a repository-relative path without '..'")
	}
	cur, ok, err := s.lookup(alias)
	if err != nil {
		return err
	}
	if ok && (cur.kind != "dockerfile" || cur.script != script) {
		return fmt.Errorf("alias %s already maps to a different image source", alias)
	}
	if !ok {
		s.addImage(alias, dockerfileNode(script))
	}
	return s.setStatus(alias, version, "init", true)
}

func badScript(script string) bool {
	if strings.HasPrefix(script, "/") {
		return true
	}
	for _, part := range strings.Split(script, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

// setStatus 写版本状态。moveFirst 为真时把该版本挪到最前。
// 登记新版本要靠这个顺序；构建回写终态时必须留在原位，避免盖掉等待期间用户调整过的顺序。
func (s *store) setStatus(alias, version, status string, moveFirst bool) error {
	if err := validateVersion(version); err != nil {
		return err
	}
	versions, err := s.versionsNode(alias)
	if err != nil {
		return err
	}
	if moveFirst {
		mapDelete(versions, version)
		openMap(versions)
		key := strNode(version)
		val := statusNode(status)
		versions.Content = append([]*yaml.Node{key, val}, versions.Content...)
		return nil
	}
	mapSet(versions, version, statusNode(status))
	return nil
}

func (s *store) versionsNode(alias string) (*yaml.Node, error) {
	img := mapGet(s.images, alias)
	if img == nil || img.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("invalid image record: %s", alias)
	}
	versions := mapGet(img, "versions")
	if versions == nil {
		versions = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		mapSet(img, "versions", versions)
	}
	if versions.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("versions must be a map: %s", alias)
	}
	return versions, nil
}

func (s *store) addImage(alias string, node *yaml.Node) {
	openMap(s.images)
	s.images.Content = append(s.images.Content, strNode(alias), node)
}

func validateAlias(alias string) error {
	if !aliasRE.MatchString(alias) {
		return fmt.Errorf("invalid alias: %s; expected a single lowercase repository name", alias)
	}
	return nil
}

func validateVersion(version string) error {
	if !versionRE.MatchString(version) {
		return fmt.Errorf("invalid Docker tag: %s", version)
	}
	return nil
}

func splitSource(raw string) (string, string, bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.Contains(value, "@") {
		return "", "", false, errors.New("source must be a non-empty tagged image name, digest references are not supported")
	}
	slash := strings.LastIndex(value, "/")
	colon := strings.LastIndex(value, ":")
	if colon > slash {
		return value[:colon], value[colon+1:], true, nil
	}
	return value, "", false, nil
}

func mirrorNode(source string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	mapSet(node, "type", strNode("mirror"))
	mapSet(node, "source", strNode(source))
	mapSet(node, "versions", &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"})
	return node
}

func dockerfileNode(script string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	mapSet(node, "type", strNode("dockerfile"))
	mapSet(node, "script", strNode(script))
	mapSet(node, "versions", &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"})
	return node
}

func statusNode(status string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	mapSet(node, "status", strNode(status))
	return node
}

func strNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func scalar(node *yaml.Node) (string, bool) {
	if node == nil || node.Kind != yaml.ScalarNode {
		return "", false
	}
	return node.Value, true
}

func mapKeys(node *yaml.Node) []string {
	seen := map[string]struct{}{}
	keys := make([]string, 0, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

func mapGet(node *yaml.Node, key string) *yaml.Node {
	if node == nil {
		return nil
	}
	var found *yaml.Node
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			found = node.Content[i+1]
		}
	}
	return found
}

func mapSet(node *yaml.Node, key string, val *yaml.Node) {
	for i := len(node.Content) - 2; i >= 0; i -= 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = val
			return
		}
	}
	openMap(node)
	node.Content = append(node.Content, strNode(key), val)
}

func mapDelete(node *yaml.Node, key string) {
	next := make([]*yaml.Node, 0, len(node.Content))
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			continue
		}
		next = append(next, node.Content[i], node.Content[i+1])
	}
	node.Content = next
}

// openMap 把空的 flow map 打开成 block。否则 `images: {}` 加第一条时会写成一行，注释和缩进都保不住。
func openMap(node *yaml.Node) {
	if len(node.Content) == 0 {
		node.Style &^= yaml.FlowStyle
	}
}
