package codews

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultPort = 7001
	portScan    = 100
)

// Runtime 是 serve 的读模型。索引在启动时建好，拓扑变化后再换一份。
type Runtime struct {
	mu    sync.Mutex
	Root  string
	Files []string
	Watch string
	hub   *wsHub
}

func (rt *Runtime) files() []string {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	out := make([]string, len(rt.Files))
	copy(out, rt.Files)
	return out
}

type filesBody struct {
	Files []string `json:"files"`
}

type metaBody struct {
	Root     string `json:"root"`
	RootPath string `json:"rootPath"`
	Watch    string `json:"watch"`
}

type treeBody struct {
	Path    string      `json:"path"`
	Entries []TreeEntry `json:"entries"`
}

func (rt *Runtime) watch() string {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.Watch
}

func (rt *Runtime) setWatch(status string) {
	rt.mu.Lock()
	rt.Watch = status
	rt.mu.Unlock()
}

func (rt *Runtime) reindex() {
	files := ListIndex(rt.Root, 30000)
	rt.mu.Lock()
	rt.Files = files
	rt.mu.Unlock()
}

// ServeHTTP 处理只读演示请求：API、raw 和 SPA。
func (rt *Runtime) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/watch" {
		if rt.watch() != "active" {
			writeText(w, http.StatusServiceUnavailable, "watch is not active")
			return
		}
		rt.hub.upgrade(w, r)
		return
	}
	handleServe(w, r, rt)
}

func handleServe(w http.ResponseWriter, r *http.Request, rt *Runtime) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeText(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := r.URL.Path
	defer func() {
		if rec := recover(); rec != nil {
			writeText(w, http.StatusBadRequest, fmt.Sprint(rec))
		}
	}()
	err := routeServe(w, r, rt, path)
	if err == nil {
		return
	}
	msg := err.Error()
	switch msg {
	case "not found":
		writeText(w, http.StatusNotFound, msg)
	case "path denied", "path escapes root", "invalid path":
		writeText(w, http.StatusForbidden, msg)
	default:
		writeText(w, http.StatusBadRequest, msg)
	}
}

func routeServe(w http.ResponseWriter, r *http.Request, rt *Runtime, path string) error {
	root := rt.Root
	switch path {
	case "/api/blob":
		blob, err := ReadBlob(root, r.URL.Query().Get("path"))
		if err != nil {
			return err
		}
		writeJSONCode(w, http.StatusOK, blob)
		return nil
	case "/api/index":
		writeJSONCode(w, http.StatusOK, filesBody{Files: rt.files()})
		return nil
	case "/api/meta":
		body := metaBody{Root: filepath.Base(root), RootPath: root, Watch: rt.watch()}
		writeJSONCode(w, http.StatusOK, body)
		return nil
	case "/api/tree":
		blob, err := ReadBlob(root, r.URL.Query().Get("path"))
		if err != nil {
			return err
		}
		if blob.Type != "dir" {
			writeText(w, http.StatusBadRequest, "not a directory")
			return nil
		}
		entries := blob.Entries
		if entries == nil {
			entries = []TreeEntry{}
		}
		writeJSONCode(w, http.StatusOK, treeBody{Path: blob.Path, Entries: entries})
		return nil
	case "/api/symbol":
		q := r.URL.Query().Get("q")
		if !symbolRe.MatchString(q) {
			writeText(w, http.StatusBadRequest, "invalid symbol")
			return nil
		}
		writeJSONCode(w, http.StatusOK, SearchSymbol(root, rt.files(), q))
		return nil
	}
	if path == "/raw" || strings.HasPrefix(path, "/raw/") {
		rel := ""
		if strings.HasPrefix(path, "/raw/") {
			rel = strings.TrimPrefix(path, "/raw/")
		}
		opened, err := OpenRaw(root, rel)
		if err != nil {
			return err
		}
		writeRaw(w, r, opened)
		return nil
	}
	wants := strings.Contains(r.Header.Get("Accept"), "text/html")
	if wants || path == "/" || !strings.HasPrefix(path, "/api/") {
		if !wants && path != "/" && !strings.HasPrefix(path, "/api/") {
			rel := strings.TrimLeft(path, "/")
			blob, err := ReadBlob(root, rel)
			if err == nil && blob.Type == "file" && blob.Binary {
				opened, err := OpenRaw(root, rel)
				if err == nil {
					writeRaw(w, r, opened)
					return nil
				}
			}
			if err == nil && blob.Type == "file" && blob.Content != nil {
				w.Header().Set("content-type", "text/plain; charset=utf-8")
				w.Header().Set("cache-control", "no-store")
				if r.Method != http.MethodHead {
					_, _ = io.WriteString(w, *blob.Content)
				}
				return nil
			}
		}
		html := RenderPage(filepath.Base(root))
		w.Header().Set("content-type", "text/html; charset=utf-8")
		w.Header().Set("cache-control", "no-store")
		if r.Method != http.MethodHead {
			_, _ = io.WriteString(w, html)
		}
		return nil
	}
	writeText(w, http.StatusNotFound, "not found")
	return nil
}

func writeText(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("content-type", "text/plain; charset=utf-8")
	w.Header().Set("cache-control", "no-store")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, msg)
}

func writeJSONCode(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	encodeJSON(&buf, v)
	body := bytes.TrimRight(buf.Bytes(), "\n")
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.Header().Set("cache-control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// ParseByteRange 解析单个 bytes 区间。视频拖动依赖 206，不能每次回整文件。
func ParseByteRange(header string, size int64) (int64, int64, bool) {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, "bytes=") || size <= 0 {
		return 0, 0, false
	}
	spec := strings.TrimPrefix(header, "bytes=")
	left, right, ok := strings.Cut(spec, "-")
	if !ok || (left == "" && right == "") {
		return 0, 0, false
	}
	if strings.Contains(left, ",") || strings.Contains(right, ",") {
		return 0, 0, false
	}
	if left == "" {
		n, err := strconv.ParseInt(right, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		start := size - n
		if start < 0 {
			start = 0
		}
		return start, size - 1, true
	}
	start, err := strconv.ParseInt(left, 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false
	}
	end := size - 1
	if right != "" {
		end, err = strconv.ParseInt(right, 10, 64)
		if err != nil || end < start {
			return 0, 0, false
		}
		if end > size-1 {
			end = size - 1
		}
	}
	return start, end, true
}

func writeRaw(w http.ResponseWriter, r *http.Request, opened RawFile) {
	w.Header().Set("content-type", opened.Type)
	w.Header().Set("cache-control", "no-store")
	w.Header().Set("accept-ranges", "bytes")
	rangeHdr := r.Header.Get("Range")
	if rangeHdr == "" {
		w.Header().Set("content-length", strconv.FormatInt(opened.Size, 10))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}
		f, err := os.Open(opened.Abs)
		if err != nil {
			return
		}
		defer f.Close()
		_, _ = io.Copy(w, f)
		return
	}
	start, end, ok := ParseByteRange(rangeHdr, opened.Size)
	if !ok {
		w.Header().Set("content-range", fmt.Sprintf("bytes */%d", opened.Size))
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		if r.Method != http.MethodHead {
			_, _ = io.WriteString(w, "invalid range")
		}
		return
	}
	w.Header().Set("content-length", strconv.FormatInt(end-start+1, 10))
	w.Header().Set("content-range", fmt.Sprintf("bytes %d-%d/%d", start, end, opened.Size))
	w.WriteHeader(http.StatusPartialContent)
	if r.Method == http.MethodHead {
		return
	}
	f, err := os.Open(opened.Abs)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = io.Copy(w, io.NewSectionReader(f, start, end-start+1))
}

type serveOpts struct {
	root  string
	host  string
	port  int
	lan   bool
	watch bool
}

// BuildServeOpts 把 CLI 语义收成监听参数。端口超出范围直接失败，不悄悄夹紧。
func BuildServeOpts(args Parsed, cwd string) (serveOpts, error) {
	path := ""
	if args.HasPath {
		path = args.Path
	}
	root, err := ResolveRoot(path, cwd)
	if err != nil {
		return serveOpts{}, err
	}
	host := "127.0.0.1"
	if args.Lan {
		host = "0.0.0.0"
	}
	port := defaultPort
	if args.HasPort {
		port = args.Port
	}
	if port < 1 || port > 65535 {
		return serveOpts{}, errors.New("port must be an integer between 1 and 65535")
	}
	return serveOpts{root: root, host: host, port: port, lan: args.Lan, watch: args.Watch}, nil
}

func addrInUse(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	msg := err.Error()
	low := strings.ToLower(msg)
	if strings.Contains(msg, "EADDRINUSE") || strings.Contains(low, "address already in use") {
		return true
	}
	return strings.Contains(low, "is port") && strings.Contains(low, "in use")
}

func scanBind(host string, start int) (net.Listener, int, error) {
	last := start + portScan - 1
	if last > 65535 {
		last = 65535
	}
	for port := start; port <= last; port++ {
		ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err == nil {
			return ln, port, nil
		}
		if !addrInUse(err) {
			return nil, 0, err
		}
	}
	return nil, 0, fmt.Errorf("no free port from %d to %d", start, last)
}

// AccessURLs 收集可打印的访问地址。局域网模式把网卡 IPv4 放在前面，避免默认打开落到 127。
func AccessURLs(host string, port int, lan bool) []string {
	if !lan && host == "127.0.0.1" {
		return []string{fmt.Sprintf("http://127.0.0.1:%d", port)}
	}
	if !lan && host == "localhost" {
		return []string{fmt.Sprintf("http://localhost:%d", port)}
	}
	var urls []string
	push := func(url string) {
		for _, item := range urls {
			if item == url {
				return
			}
		}
		urls = append(urls, url)
	}
	if lan || host == "0.0.0.0" || host == "::" {
		for _, ip := range listLan() {
			push(fmt.Sprintf("http://%s:%d", ip, port))
		}
	}
	if host != "0.0.0.0" && host != "::" {
		push(fmt.Sprintf("http://%s:%d", host, port))
	}
	push(fmt.Sprintf("http://127.0.0.1:%d", port))
	return urls
}

type ifaceKind int

const (
	kindWiFi ifaceKind = iota + 1
	kindEthernet
	kindOther
)

// macosHardwarePorts 查询系统硬件端口映射表，用于解决 macOS 下 en0/en1 无法单纯通过名称区分无线与有线网卡的问题。
func macosHardwarePorts() map[string]string {
	if runtime.GOOS != "darwin" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "networksetup", "-listallhardwareports")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	ports := make(map[string]string)
	var curPort string
	for _, rawLine := range strings.Split(string(out), "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "Hardware Port:") {
			curPort = strings.TrimSpace(strings.TrimPrefix(line, "Hardware Port:"))
		} else if strings.HasPrefix(line, "Device:") && curPort != "" {
			dev := strings.TrimSpace(strings.TrimPrefix(line, "Device:"))
			ports[dev] = curPort
			curPort = ""
		}
	}
	return ports
}

// classifyInterface 识别网卡物理类型，保证优先从局域网 WiFi 地址启动预览，便于移动端真机同网段联调。
func classifyInterface(iface net.Interface, macPorts map[string]string) ifaceKind {
	name := strings.ToLower(iface.Name)
	for _, pfx := range []string{"utun", "tun", "tap", "docker", "veth", "br-", "virbr", "awdl", "llw", "p2p", "bridge", "vmnet", "vboxnet", "tailscale", "wg"} {
		if strings.HasPrefix(name, pfx) {
			return kindOther
		}
	}

	switch runtime.GOOS {
	case "darwin":
		if macPorts != nil {
			if port, ok := macPorts[iface.Name]; ok {
				p := strings.ToLower(port)
				if strings.Contains(p, "wi-fi") || strings.Contains(p, "airport") || strings.Contains(p, "wireless") {
					return kindWiFi
				}
				if strings.Contains(p, "ethernet") || strings.Contains(p, "lan") || strings.Contains(p, "thunderbolt") {
					return kindEthernet
				}
			}
		}
		if name == "en0" || strings.Contains(name, "wifi") || strings.Contains(name, "wlan") {
			return kindWiFi
		}
		if strings.HasPrefix(name, "en") || strings.HasPrefix(name, "eth") {
			return kindEthernet
		}

	case "linux":
		// Linux sysfs: /sys/class/net/<name>/wireless 存在即为无线物理网卡
		if _, err := os.Stat("/sys/class/net/" + iface.Name + "/wireless"); err == nil {
			return kindWiFi
		}
		if strings.HasPrefix(name, "wl") || strings.Contains(name, "wifi") || strings.Contains(name, "wlan") {
			return kindWiFi
		}
		if strings.HasPrefix(name, "eth") || strings.HasPrefix(name, "en") || strings.HasPrefix(name, "em") ||
			strings.HasPrefix(name, "eno") || strings.HasPrefix(name, "enp") || strings.HasPrefix(name, "ens") {
			return kindEthernet
		}

	case "windows":
		// Windows: net.Interfaces() 返回本地网络连接名（如 "Wi-Fi", "以太网"）
		if strings.Contains(name, "wi-fi") || strings.Contains(name, "wifi") || strings.Contains(name, "wlan") || strings.Contains(name, "wireless") || strings.Contains(name, "无线") {
			if !strings.Contains(name, "virtual") && !strings.Contains(name, "vethernet") && !strings.Contains(name, "tap") {
				return kindWiFi
			}
		}
		if strings.Contains(name, "ethernet") || strings.Contains(name, "以太网") || strings.Contains(name, "local area connection") || strings.Contains(name, "本地连接") || strings.Contains(name, "eth") {
			if !strings.Contains(name, "virtual") && !strings.Contains(name, "vethernet") && !strings.Contains(name, "loopback") {
				return kindEthernet
			}
		}

	default:
		if strings.Contains(name, "wifi") || strings.Contains(name, "wlan") || strings.HasPrefix(name, "wl") {
			return kindWiFi
		}
		if strings.HasPrefix(name, "eth") || strings.HasPrefix(name, "en") {
			return kindEthernet
		}
	}

	return kindOther
}

// listLan 收集并按优先级排序本机局域网 IPv4 地址：WiFi 优先，其次有线 Ethernet，最后其他虚拟/隧道网卡。
func listLan() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	macPorts := macosHardwarePorts()

	var wifiIPs []string
	var ethIPs []string
	var otherPrivateIPs []string
	var otherIPs []string

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		kind := classifyInterface(iface, macPorts)

		for _, addr := range addrs {
			netIP, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := netIP.IP.To4()
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			ipStr := ip.String()

			switch kind {
			case kindWiFi:
				wifiIPs = append(wifiIPs, ipStr)
			case kindEthernet:
				ethIPs = append(ethIPs, ipStr)
			default:
				if ip.IsPrivate() {
					otherPrivateIPs = append(otherPrivateIPs, ipStr)
				} else {
					otherIPs = append(otherIPs, ipStr)
				}
			}
		}
	}

	var result []string
	seen := make(map[string]bool)
	add := func(ips []string) {
		for _, ip := range ips {
			if !seen[ip] {
				seen[ip] = true
				result = append(result, ip)
			}
		}
	}

	add(wifiIPs)
	add(ethIPs)
	add(otherPrivateIPs)
	add(otherIPs)

	return result
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Start()
}

func runServe(out, errw io.Writer, args Parsed) error {
	opts, err := BuildServeOpts(args, cwd())
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "code-ws serve")
	fmt.Fprintf(out, "  root:  %s\n", opts.root)
	fmt.Fprintln(out, "  index: building...")
	started := time.Now()
	files := ListIndex(opts.root, 30000)
	rt := &Runtime{Root: opts.root, Files: files, Watch: "disabled", hub: newHub()}
	fmt.Fprintf(out, "  index: %d files (%dms)\n", len(files), time.Since(started).Milliseconds())

	var ctl *WatchCtl
	if opts.watch {
		rt.Watch = "active"
		ctl = StartWatch(watchHooks{
			Root: opts.root,
			OnTopology: func() {
				rt.reindex()
			},
			OnEvent: func(ev WatchEvent) {
				var buf bytes.Buffer
				encodeJSON(&buf, ev)
				rt.hub.publish(bytes.TrimRight(buf.Bytes(), "\n"))
			},
			OnError: func(err error) {
				fmt.Fprintf(errw, "code-ws: watch failed: %s\n", err.Error())
			},
		})
	}
	ln, port, err := scanBind(opts.host, opts.port)
	if err != nil {
		if ctl != nil {
			ctl.Close()
		}
		return err
	}
	srv := &http.Server{Handler: rt}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	if port != opts.port {
		fmt.Fprintf(out, "  bind:  %s:%d (%d in use)\n", opts.host, port, opts.port)
	} else {
		fmt.Fprintf(out, "  bind:  %s:%d\n", opts.host, port)
	}
	fmt.Fprintf(out, "  watch: %s\n", rt.watch())
	urls := AccessURLs(opts.host, port, opts.lan)
	for _, url := range urls {
		fmt.Fprintf(out, "  url:   %s\n", url)
	}
	if opts.lan {
		fmt.Fprintln(out, "  note:  no auth; deny-list only. stop with Ctrl+C")
	}
	if len(urls) == 0 {
		if ctl != nil {
			ctl.Close()
		}
		_ = srv.Close()
		return errors.New("no access url")
	}
	openBrowser(urls[0])
	if !args.Cf {
		err := <-serveErr
		if ctl != nil {
			ctl.Close()
		}
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
	fmt.Fprintln(out, "  cf:    requesting quick tunnel")
	stop := make(chan error, 1)
	tun := StartQuick(port, func(url string) {
		fmt.Fprintf(out, "  cf:    %s\n", url)
	}, func(msg string) {
		stop <- errors.New(msg)
	})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)
	var ret error
	select {
	case err := <-serveErr:
		ret = err
	case err := <-stop:
		ret = err
	case s := <-sig:
		code := 143
		if s == syscall.SIGINT {
			code = 130
		}
		ret = &Exit{Code: code}
	}
	tun.Close()
	if ctl != nil {
		ctl.Close()
	}
	_ = srv.Close()
	if ret == nil || errors.Is(ret, http.ErrServerClosed) {
		return nil
	}
	return ret
}
