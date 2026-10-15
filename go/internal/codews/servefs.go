package codews

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const textMax = 2 * 1024 * 1024
const indexMaxDepth = 64

// TreeEntry 是侧栏和 /api/tree 的一项。
type TreeEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

// Blob 是目录或文件的只读载荷。文本才带 Content，避免把二进制塞进 JSON。
type Blob struct {
	Path    string      `json:"path"`
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Size    int64       `json:"size"`
	Binary  bool        `json:"binary"`
	Lang    string      `json:"language"`
	Entries []TreeEntry `json:"entries,omitempty"`
	Readme  *string     `json:"readme,omitempty"`
	Content *string     `json:"content,omitempty"`
	Preview string      `json:"preview,omitempty"`
}

// RawFile 只给出路径和长度。视频要按 Range 切，不能先把整文件读进内存。
type RawFile struct {
	Abs  string
	Size int64
	Type string
}

var denyDirs = map[string]bool{".git": true, ".svn": true, ".hg": true, ".ssh": true}

var skipDirs = map[string]bool{
	"node_modules": true, "target": true, "build": true, "dist": true, "out": true,
	"classes": true, ".gradle": true, ".idea": true, ".vscode": true, "vendor": true,
	"coverage": true, "__pycache__": true, ".next": true, ".nuxt": true, ".turbo": true,
	".cache": true, ".terraform": true, "Pods": true, "bower_components": true,
}

var skipExts = map[string]bool{
	".class": true, ".jar": true, ".war": true, ".ear": true, ".o": true, ".a": true,
	".so": true, ".dylib": true, ".dll": true, ".exe": true, ".bin": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".ico": true, ".bmp": true,
	".zip": true, ".tar": true, ".gz": true, ".tgz": true, ".7z": true, ".rar": true,
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true, ".map": true,
	".min.js": true, ".min.css": true,
}

var prioExts = map[string]bool{
	".md": true, ".markdown": true, ".kt": true, ".kts": true, ".java": true,
	".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".go": true, ".rs": true, ".py": true, ".json": true, ".yml": true, ".yaml": true,
	".toml": true, ".xml": true, ".gradle": true, ".sql": true, ".proto": true,
	".graphql": true, ".sh": true, ".bash": true, ".zsh": true, ".fish": true,
	".css": true, ".scss": true, ".html": true, ".txt": true,
}

var denyRes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\.env(\..+)?$`),
	regexp.MustCompile(`(?i)^id_rsa`),
	regexp.MustCompile(`(?i)^id_ed25519`),
	regexp.MustCompile(`(?i)^id_dsa`),
	regexp.MustCompile(`(?i)\.pem$`),
	regexp.MustCompile(`(?i)\.p12$`),
	regexp.MustCompile(`(?i)\.pfx$`),
	regexp.MustCompile(`(?i)\.key$`),
	regexp.MustCompile(`(?i)secret`),
	regexp.MustCompile(`(?i)credentials`),
}

var langByExt = map[string]string{
	".ts": "typescript", ".tsx": "tsx", ".js": "javascript", ".jsx": "jsx",
	".mjs": "javascript", ".cjs": "javascript", ".json": "json", ".md": "markdown",
	".markdown": "markdown", ".yml": "yaml", ".yaml": "yaml", ".toml": "toml",
	".rs": "rust", ".go": "go", ".py": "python", ".java": "java", ".kt": "kotlin",
	".kts": "kotlin", ".css": "css", ".scss": "scss", ".html": "html", ".htm": "html",
	".xml": "xml", ".sh": "bash", ".bash": "bash", ".zsh": "bash", ".fish": "bash",
	".sql": "sql", ".graphql": "graphql", ".proto": "protobuf", ".c": "c", ".h": "c",
	".cpp": "cpp", ".cc": "cpp", ".hpp": "cpp", ".swift": "swift", ".rb": "ruby",
	".php": "php", ".lua": "lua", ".r": "r", ".dockerfile": "dockerfile",
	".txt": "plaintext", ".svg": "xml",
}

var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".svg": true, ".ico": true, ".bmp": true,
}

var videoExts = map[string]bool{".mp4": true, ".webm": true, ".mov": true, ".m4v": true, ".ogv": true}
var audioExts = map[string]bool{".mp3": true, ".wav": true, ".ogg": true, ".m4a": true, ".flac": true, ".aac": true}

var bareNames = map[string]bool{
	"dockerfile": true, "makefile": true, "justfile": true, "procfile": true,
	"gemfile": true, "rakefile": true,
}

func deniedName(name string) bool {
	if denyDirs[name] {
		return true
	}
	for _, re := range denyRes {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

// DeniedRel 用路径闸门代替 token。无鉴权的演示服务不能把密钥目录露出去。
func DeniedRel(rel string) bool {
	rel = strings.ReplaceAll(rel, "\\", "/")
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." {
			continue
		}
		if part == ".." || deniedName(part) {
			return true
		}
	}
	return false
}

// WatchIgnored 跟索引共用噪声目录，避免构建产物把浏览器推送打满。
func WatchIgnored(rel string) bool {
	clean := strings.ReplaceAll(rel, "\\", "/")
	if DeniedRel(clean) {
		return true
	}
	for _, part := range strings.Split(clean, "/") {
		if part != "" && skipDirs[part] {
			return true
		}
	}
	return false
}

func absClean(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return a
}

// ResolveRoot 把缺省 cwd 和相对 path 收成绝对目录。
func ResolveRoot(pathArg, cwd string) (string, error) {
	raw := pathArg
	if raw == "" {
		raw = cwd
	}
	abs := raw
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(cwd, abs)
	}
	abs = absClean(abs)
	st, err := os.Stat(abs)
	if err != nil {
		return "", errPath("serve path not found: " + abs)
	}
	if !st.IsDir() {
		return "", errPath("serve path is not a directory: " + abs)
	}
	return abs, nil
}

type errPath string

func (e errPath) Error() string { return string(e) }

// UnderRoot 把 URL 里的相对路径钉在 root 内，挡住 path traversal。
func UnderRoot(root, rel string) (string, error) {
	cleaned := strings.ReplaceAll(rel, "\\", "/")
	cleaned = strings.Trim(cleaned, "/")
	if strings.Contains(cleaned, "\x00") {
		return "", errPath("invalid path")
	}
	if DeniedRel(cleaned) {
		return "", errPath("path denied")
	}
	rootAbs := absClean(root)
	abs := rootAbs
	if cleaned != "" {
		abs = absClean(filepath.Join(rootAbs, filepath.FromSlash(cleaned)))
	}
	sep := string(filepath.Separator)
	if abs != rootAbs && !strings.HasPrefix(abs, rootAbs+sep) {
		return "", errPath("path escapes root")
	}
	if DeniedRel(PosixRel(rootAbs, abs)) {
		return "", errPath("path denied")
	}
	return abs, nil
}

// PosixRel 统一成斜杠相对路径，深链和索引用同一格式。
func PosixRel(root, abs string) string {
	rel, err := filepath.Rel(absClean(root), absClean(abs))
	if err != nil || rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

// PreviewKind 由扩展名决定预览器。xlsx/pdf 不能只因为是二进制就退化成下载。
func PreviewKind(rel string, binary bool) string {
	ext := strings.ToLower(filepath.Ext(rel))
	switch {
	case imageExts[ext]:
		return "image"
	case ext == ".csv":
		return "csv"
	case ext == ".tsv":
		return "tsv"
	case ext == ".xlsx" || ext == ".xls":
		return "xlsx"
	case ext == ".pdf":
		return "pdf"
	case ext == ".docx":
		return "docx"
	case videoExts[ext]:
		return "video"
	case audioExts[ext]:
		return "audio"
	case ext == ".html" || ext == ".htm":
		return "html"
	case ext == ".jsonl" || ext == ".ndjson":
		return "jsonl"
	case ext == ".md" || ext == ".markdown":
		return "markdown"
	case binary:
		return "binary"
	default:
		return "code"
	}
}

// LanguageOf 给高亮用。Dockerfile 这类没扩展名的文件单独认。
func LanguageOf(rel string) string {
	base := strings.ToLower(filepath.Base(rel))
	if base == "dockerfile" {
		return "dockerfile"
	}
	if base == "makefile" {
		return "makefile"
	}
	ext := strings.ToLower(filepath.Ext(base))
	if lang, ok := langByExt[ext]; ok {
		return lang
	}
	return "plaintext"
}

func readmeScore(name string) int {
	n := strings.ToLower(name)
	switch n {
	case "readme.md":
		return 100
	case "readme.markdown":
		return 90
	case "readme.mdown":
		return 80
	case "readme.mkd":
		return 70
	case "readme.rst":
		return 60
	case "readme.txt":
		return 50
	case "readme":
		return 40
	}
	if strings.HasPrefix(n, "readme.") {
		return 30
	}
	return -1
}

// PickReadme 按 GitHub 的优先序挑目录说明，首页要同时给树和文档。
func PickReadme(entries []TreeEntry) string {
	best := ""
	score := -1
	for _, ent := range entries {
		if ent.Type != "file" {
			continue
		}
		cur := readmeScore(ent.Name)
		if cur > score {
			best = ent.Path
			score = cur
		}
	}
	return best
}

func listEntries(root, dir string) []TreeEntry {
	names, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var entries []TreeEntry
	for _, ent := range names {
		name := ent.Name()
		if deniedName(name) {
			continue
		}
		abs := filepath.Join(dir, name)
		rel := PosixRel(root, abs)
		if DeniedRel(rel) {
			continue
		}
		st, err := os.Lstat(abs)
		if err != nil || st.Mode()&os.ModeSymlink != 0 {
			continue
		}
		kind := ""
		if st.IsDir() {
			kind = "dir"
		} else if st.Mode().IsRegular() {
			kind = "file"
		} else {
			continue
		}
		entries = append(entries, TreeEntry{Name: name, Path: rel, Type: kind})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == "dir"
		}
		return entries[i].Name < entries[j].Name
	})
	return entries
}

// ReadBlob 读目录或文件。超过文本上限或像二进制的文件不内联内容。
func ReadBlob(root, rel string) (Blob, error) {
	abs, err := UnderRoot(root, rel)
	if err != nil {
		return Blob{}, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return Blob{}, errPath("not found")
	}
	path := PosixRel(root, abs)
	name := filepath.Base(abs)
	if path == "" {
		name = filepath.Base(root)
	}
	if st.IsDir() {
		entries := listEntries(root, abs)
		blob := Blob{
			Path: path, Name: name, Type: "dir", Size: 0, Binary: false,
			Lang: "plaintext", Entries: entries,
		}
		if blob.Entries == nil {
			blob.Entries = []TreeEntry{}
		}
		if readme := PickReadme(entries); readme != "" {
			blob.Readme = &readme
		}
		return blob, nil
	}
	if !st.Mode().IsRegular() {
		return Blob{}, errPath("unsupported file type")
	}
	size := st.Size()
	ext := strings.ToLower(filepath.Ext(path))
	image := imageExts[ext]
	binary := image || size > textMax
	var content *string
	if !image && size <= textMax {
		buf, err := os.ReadFile(abs)
		if err != nil {
			return Blob{}, err
		}
		if LooksBinary(buf) {
			binary = true
		} else {
			text := strings.ToValidUTF8(string(buf), "�")
			content = &text
		}
	}
	return Blob{
		Path: path, Name: name, Type: "file", Size: size, Binary: binary,
		Lang: langOrImage(path, image), Content: content, Preview: PreviewKind(path, binary),
	}, nil
}

func langOrImage(path string, image bool) string {
	if image {
		return "image"
	}
	return LanguageOf(path)
}

// LooksBinary 用 NUL 和控制字符比例挡乱码，不把二进制当源码渲染。
func LooksBinary(buf []byte) bool {
	if len(buf) == 0 {
		return false
	}
	sample := buf
	if len(sample) > 8000 {
		sample = sample[:8000]
	}
	bad := 0
	for _, b := range sample {
		if b == 0 {
			return true
		}
		if b < 7 || (b > 13 && b < 32) {
			bad++
		}
	}
	return float64(bad)/float64(len(sample)) > 0.3
}

func indexable(name string) bool {
	lower := strings.ToLower(name)
	ext := filepath.Ext(lower)
	if skipExts[ext] {
		return false
	}
	if ext == "" && !bareNames[lower] {
		return strings.HasPrefix(lower, "readme") || strings.HasPrefix(lower, "license") || strings.HasPrefix(lower, "changelog")
	}
	return true
}

func priorityFile(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "readme") || lower == "agents.md" {
		return true
	}
	return prioExts[filepath.Ext(lower)]
}

// ListIndex 收集可检索路径。源码和文档优先，避免 上限被 .class 占满后 README 消失。
func ListIndex(root string, maxFiles int) []string {
	if maxFiles <= 0 {
		maxFiles = 30000
	}
	var prio, rest []string
	seen := map[string]bool{}
	walkIndex(root, root, &prio, &rest, maxFiles, 0, seen)
	out := prio
	if len(prio) >= maxFiles {
		out = append([]string{}, prio[:maxFiles]...)
	} else {
		room := maxFiles - len(prio)
		if room > len(rest) {
			room = len(rest)
		}
		out = append(append([]string{}, prio...), rest[:room]...)
	}
	sort.Strings(out)
	if out == nil {
		out = []string{}
	}
	return out
}

func walkIndex(root, dir string, prio, rest *[]string, maxFiles, depth int, seen map[string]bool) {
	if len(*prio) >= maxFiles || depth > indexMaxDepth {
		return
	}
	key := absClean(dir)
	if seen[key] {
		return
	}
	seen[key] = true
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	names := make([]string, 0, len(ents))
	for _, ent := range ents {
		names = append(names, ent.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		if len(*prio) >= maxFiles {
			return
		}
		if denyDirs[name] || skipDirs[name] || deniedName(name) {
			continue
		}
		abs := filepath.Join(dir, name)
		rel := PosixRel(root, abs)
		if DeniedRel(rel) {
			continue
		}
		st, err := os.Lstat(abs)
		if err != nil || st.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if st.IsDir() {
			walkIndex(root, abs, prio, rest, maxFiles, depth+1, seen)
			continue
		}
		if !st.Mode().IsRegular() || !indexable(name) {
			continue
		}
		if priorityFile(name) {
			*prio = append(*prio, rel)
		} else if len(*prio)+len(*rest) < maxFiles {
			*rest = append(*rest, rel)
		}
	}
}

// OpenRaw 定位文件但不读内容。
func OpenRaw(root, rel string) (RawFile, error) {
	abs, err := UnderRoot(root, rel)
	if err != nil {
		return RawFile{}, err
	}
	st, err := os.Stat(abs)
	if err != nil || !st.Mode().IsRegular() {
		return RawFile{}, errPath("not found")
	}
	return RawFile{Abs: abs, Size: st.Size(), Type: contentType(rel)}, nil
}

// ReadIndexText 只读小文本。超限和二进制直接跳过，一次点击不能把进程读满。
func ReadIndexText(root, rel string, maxBytes int64) (string, bool) {
	abs, err := UnderRoot(root, rel)
	if err != nil {
		return "", false
	}
	st, err := os.Stat(abs)
	if err != nil || !st.Mode().IsRegular() || st.Size() > maxBytes {
		return "", false
	}
	buf, err := os.ReadFile(abs)
	if err != nil || LooksBinary(buf) {
		return "", false
	}
	if !utf8.Valid(buf) {
		return strings.ToValidUTF8(string(buf), "�"), true
	}
	return string(buf), true
}

func contentType(rel string) string {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".bmp":
		return "image/bmp"
	case ".pdf":
		return "application/pdf"
	case ".csv":
		return "text/csv; charset=utf-8"
	case ".tsv":
		return "text/tab-separated-values; charset=utf-8"
	case ".jsonl", ".ndjson":
		return "application/x-ndjson; charset=utf-8"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".ogv":
		return "video/ogg"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".m4a":
		return "audio/mp4"
	case ".flac":
		return "audio/flac"
	case ".aac":
		return "audio/aac"
	case ".json":
		return "application/json; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".md", ".markdown", ".txt":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
