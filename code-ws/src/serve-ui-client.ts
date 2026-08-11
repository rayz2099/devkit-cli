/**
 * serve UI 浏览器端脚本, 从 serve-ui 拆出以控制单文件行数.
 */
export function renderServeClientScript(rootJson: string): string {
  return `
    const ICO = {
      folder: '<svg viewBox="0 0 16 16"><path d="M1.75 1A1.75 1.75 0 0 0 0 2.75v10.5C0 14.216.784 15 1.75 15h12.5A1.75 1.75 0 0 0 16 13.25v-8.5A1.75 1.75 0 0 0 14.25 3H7.5a.25.25 0 0 1-.2-.1l-.9-1.2C6.07 1.26 5.55 1 5 1H1.75Z"/></svg>',
      file: '<svg viewBox="0 0 16 16"><path d="M2 1.75C2 .784 2.784 0 3.75 0h6.586c.464 0 .909.184 1.237.513l2.914 2.914c.329.328.513.773.513 1.237v9.586A1.75 1.75 0 0 1 13.25 16h-9.5A1.75 1.75 0 0 1 2 14.25Zm1.75-.25a.25.25 0 0 0-.25.25v12.5c0 .138.112.25.25.25h9.5a.25.25 0 0 0 .25-.25V6h-2.75A1.75 1.75 0 0 1 9 4.25V1.5Zm6.75.062V4.25c0 .138.112.25.25.25h2.688l-.011-.013-2.914-2.914-.013-.011Z"/></svg>',
      chevron: '<svg viewBox="0 0 16 16"><path d="M6.22 3.22a.75.75 0 0 1 1.06 0l4.25 4.25a.75.75 0 0 1 0 1.06l-4.25 4.25a.75.75 0 0 1-1.06-1.06L9.94 8 6.22 4.28a.75.75 0 0 1 0-1.06Z"/></svg>',
    };

    const state = {
      rootName: ${rootJson},
      rootEntries: [],
      // dirPath -> entries; 懒加载, 避免 monorepo 一次打爆侧栏.
      treeKids: Object.create(null),
      // 展开集合用 object, 免 Set 序列化问题.
      expanded: Object.create(null),
      files: [],
      currentPath: "",
      paletteOpen: false,
      paletteItems: [],
      paletteIndex: 0,
      paletteMode: "files",
      toastTimer: 0,
      currentType: "",
      watchStatus: "disabled",
      watchSocket: null,
      watchTimer: 0,
      watchRetry: 0,
      watchSeen: false,
      watchQueue: Promise.resolve(),
    };

    const RECENT_KEY = "code-ws.serve.recent.v1";
    const RECENT_LIMIT = 20;

    function loadRecent() {
      try {
        const raw = localStorage.getItem(RECENT_KEY);
        const arr = raw ? JSON.parse(raw) : [];
        return Array.isArray(arr) ? arr.filter((x) => typeof x === "string") : [];
      } catch {
        return [];
      }
    }

    function pushRecent(path) {
      if (!path) return;
      const next = [path, ...loadRecent().filter((p) => p !== path)].slice(0, RECENT_LIMIT);
      try { localStorage.setItem(RECENT_KEY, JSON.stringify(next)); } catch {}
    }

    mermaid.initialize({ startOnLoad: false, theme: "default", securityLevel: "strict" });

    function $(id) { return document.getElementById(id); }

    function escapeHtml(s) {
      return String(s)
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;");
    }

    function encodePath(p) {
      if (!p) return "";
      return p.split("/").map(encodeURIComponent).join("/");
    }

    function currentRelPath() {
      const raw = location.pathname.replace(/^\\/+/, "");
      try { return decodeURIComponent(raw); } catch { return raw; }
    }

    function toast(msg) {
      const el = $("toast");
      el.textContent = msg;
      el.classList.add("show");
      clearTimeout(state.toastTimer);
      state.toastTimer = setTimeout(() => el.classList.remove("show"), 1400);
    }

    function paintWatchStatus(status) {
      const el = $("watch-state");
      el.classList.remove("green", "warn");
      if (status === "disabled") {
        el.hidden = true;
        return;
      }
      el.hidden = false;
      if (status === "active") {
        el.textContent = "watch active";
        el.classList.add("green");
        return;
      }
      if (status === "unavailable") {
        el.textContent = "watch unavailable";
        el.classList.add("warn");
        return;
      }
      el.textContent = status === "disconnected" ? "watch disconnected" : "watch connecting";
      el.classList.add("warn");
    }

    async function api(url) {
      const res = await fetch(url);
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || res.statusText);
      }
      return res.json();
    }

    async function ensureTree(dirPath) {
      const key = dirPath || "";
      if (state.treeKids[key]) return state.treeKids[key];
      const data = await api("/api/tree?path=" + encodeURIComponent(key));
      state.treeKids[key] = data.entries || [];
      return state.treeKids[key];
    }

    function renderTreeNodes(entries, depth) {
      return (entries || []).map((e) => {
        const active = e.path === state.currentPath ? " active" : "";
        const pad = 8 + depth * 12;
        if (e.type === "dir") {
          const open = !!state.expanded[e.path];
          const kids = open ? (state.treeKids[e.path] || []) : [];
          const row = '<div class="tree-item' + active + '" data-path="' + escapeHtml(e.path)
            + '" style="padding-left:' + pad + 'px">'
            + '<button type="button" class="tree-toggle' + (open ? " open" : "")
            + '" data-tree-toggle="' + escapeHtml(e.path)
            + '" aria-label="Toggle folder">' + ICO.chevron + "</button>"
            + '<a class="tree-link" href="/' + encodePath(e.path) + '">'
            + '<span class="ico">' + ICO.folder + '</span><span class="name">'
            + escapeHtml(e.name) + "</span></a></div>"
            + (open ? renderTreeNodes(kids, depth + 1) : "");
          return row;
        }
        return '<div class="tree-item' + active + '" data-path="' + escapeHtml(e.path)
          + '" style="padding-left:' + pad + 'px">'
          + '<span class="tree-toggle spacer" aria-hidden="true"></span>'
          + '<a class="tree-link" href="/' + encodePath(e.path) + '">'
          + '<span class="ico">' + ICO.file + '</span><span class="name">'
          + escapeHtml(e.name) + "</span></a></div>";
      }).join("");
    }

    function paintTree() {
      const list = state.treeKids[""] || state.rootEntries || [];
      $("tree").innerHTML =
        '<div class="sidebar-head"><span>Files</span><span class="badge">' + list.length + "</span></div>"
        + renderTreeNodes(list, 0);
    }

    // 深链/跳转时沿祖先展开并滚到当前项, 侧栏与主区路径对齐.
    async function revealTree(path) {
      await ensureTree("");
      if (path) {
        const parts = path.split("/");
        let acc = [];
        for (let i = 0; i < parts.length - 1; i += 1) {
          acc.push(parts[i]);
          const dir = acc.join("/");
          state.expanded[dir] = true;
          await ensureTree(dir);
        }
      }
      paintTree();
      requestAnimationFrame(() => {
        const active = document.querySelector(".tree-item.active");
        if (active) active.scrollIntoView({ block: "nearest" });
      });
    }

    function setActiveTree(path) {
      for (const el of document.querySelectorAll(".tree-item")) {
        el.classList.toggle("active", el.dataset.path === path);
      }
    }

    function renderCrumbs(path) {
      const parts = path ? path.split("/") : [];
      let acc = [];
      let html = '<nav class="pathbar"><a href="/">root</a>';
      parts.forEach((part, i) => {
        acc.push(part);
        const full = acc.join("/");
        html += '<span class="slash">/</span>';
        if (i === parts.length - 1) {
          html += '<span class="current">' + escapeHtml(part) + "</span>";
        } else {
          html += '<a href="/' + encodePath(full) + '">' + escapeHtml(part) + "</a>";
        }
      });
      return html + "</nav>";
    }

    function hasHljs() {
      return typeof hljs !== "undefined" && hljs && typeof hljs.highlight === "function";
    }

    function highlightCode(code, lang) {
      if (!hasHljs()) {
        return escapeHtml(code);
      }
      if (lang && hljs.getLanguage(lang)) {
        return hljs.highlight(code, { language: lang }).value;
      }
      return hljs.highlightAuto(code).value;
    }

    function highlightBlocks(root) {
      if (!hasHljs() || !root) return;
      root.querySelectorAll("pre code").forEach((block) => hljs.highlightElement(block));
    }

    function renderMarkdown(src) {
      const renderer = new marked.Renderer();
      const rawCode = renderer.code.bind(renderer);
      renderer.code = function(token) {
        const lang = ((token && token.lang) || "").trim();
        if (lang === "mermaid") {
          return '<div class="mermaid">' + escapeHtml(token.text) + "</div>";
        }
        return rawCode(token);
      };
      return marked.parse(src, { gfm: true, renderer });
    }

    function renderCodeView(content, lang) {
      const lines = String(content).replace(/\\n$/, "").split("\\n");
      const rows = lines.map((line) => {
        const html = highlightCode(line.length ? line : " ", lang);
        return '<tr><td class="ln"></td><td class="code"><span class="hljs">' + html + "</span></td></tr>";
      });
      // line numbers after map to keep highlight per-line simple
      const numbered = rows.map((row, i) => row.replace(
        '<td class="ln"></td>',
        '<td class="ln">' + (i + 1) + "</td>",
      )).join("");
      return '<div class="code-wrap"><table class="code-table"><tbody>' + numbered + "</tbody></table></div>";
    }

    async function renderBlob(path, opts) {
      opts = opts || {};
      const content = $("content");
      const maxScroll = Math.max(document.documentElement.scrollHeight - window.innerHeight, 0);
      const scrollRatio = maxScroll > 0 ? window.scrollY / maxScroll : 0;
      content.innerHTML = '<div class="empty">Loading…</div>';
      try {
        const data = await api("/api/blob?path=" + encodeURIComponent(path));
        state.currentPath = data.path || "";
        state.currentType = data.type || "";
        if (state.currentPath) pushRecent(state.currentPath);
        if (data.type === "dir" && !path) {
          state.rootEntries = data.entries || [];
          state.treeKids[""] = state.rootEntries;
        }
        await revealTree(state.currentPath);

        if (data.type === "dir") {
          const rows = (data.entries || []).map((e) => {
            const ico = e.type === "dir" ? ICO.folder : ICO.file;
            const cls = e.type === "dir" ? "dir-ico folder" : "dir-ico";
            return '<tr><td><div class="name-cell"><span class="' + cls + '">' + ico
              + '</span><a href="/' + encodePath(e.path) + '">' + escapeHtml(e.name)
              + (e.type === "dir" ? "/" : "") + "</a></div></td>"
              + '<td class="type-cell">' + (e.type === "dir" ? "dir" : "file") + "</td></tr>";
          }).join("");
          // GitHub 目录页语义: 目录树 + 同层 README, 路由仍停在目录本身.
          content.innerHTML = renderCrumbs(data.path)
            + '<div class="file-box"><div class="file-box-head"><div class="title">'
            + '<span class="dir-ico folder">' + ICO.folder + "</span>"
            + '<span class="name">' + escapeHtml(data.name || state.rootName) + "</span></div>"
            + '<div class="meta">' + (data.entries || []).length + " items</div></div>"
            + '<table class="dir-table"><tbody>' + rows + "</tbody></table></div>"
            + '<div id="readme-slot"></div>';
          const readmePath = data.readme || "";
          if (readmePath) {
            try {
              const rd = await api("/api/blob?path=" + encodeURIComponent(readmePath));
              const slot = $("readme-slot");
              if (slot && rd.type === "file" && !rd.binary && rd.content !== undefined) {
                const head = '<div style="height:16px"></div><div class="file-box">'
                  + '<div class="file-box-head"><div class="title">'
                  + '<span class="dir-ico">' + ICO.file + "</span>"
                  + '<span class="name">' + escapeHtml(rd.name) + "</span></div>"
                  + '<div class="meta">README</div>'
                  + '<div class="actions"><a class="btn" href="/' + encodePath(rd.path) + '">Open</a></div></div>';
                if (rd.language === "markdown") {
                  slot.innerHTML = head + '<article class="md">' + renderMarkdown(rd.content) + "</article></div>";
                  highlightBlocks(slot);
                  const nodes = slot.querySelectorAll(".mermaid");
                  if (nodes.length) await mermaid.run({ nodes });
                } else {
                  slot.innerHTML = head + renderCodeView(rd.content, rd.language || "plaintext") + "</div>";
                }
              }
            } catch (readmeErr) {
              const slot = $("readme-slot");
              if (slot) {
                slot.innerHTML = '<div style="height:16px"></div><div class="file-box"><div class="empty">README: '
                  + escapeHtml(readmeErr.message || String(readmeErr)) + "</div></div>";
              }
            }
          }
          return;
        }

        if (data.language === "image") {
          content.innerHTML = renderCrumbs(data.path)
            + '<div class="file-box"><div class="file-box-head"><div class="title">'
            + '<span class="dir-ico">' + ICO.file + "</span>"
            + '<span class="name">' + escapeHtml(data.name) + "</span></div>"
            + '<div class="meta">' + data.size + " bytes</div>"
            + '<div class="actions"><a class="btn" href="/raw/' + encodePath(data.path) + '">Raw</a></div></div>'
            + '<div class="img-wrap"><img src="/raw/' + encodePath(data.path)
            + '" alt="' + escapeHtml(data.name) + '" /></div></div>';
          return;
        }

        if (data.binary || data.content === undefined) {
          content.innerHTML = renderCrumbs(data.path)
            + '<div class="file-box"><div class="file-box-head"><div class="title">'
            + '<span class="dir-ico">' + ICO.file + "</span>"
            + '<span class="name">' + escapeHtml(data.name) + "</span></div>"
            + '<div class="meta">binary · ' + data.size + " bytes</div>"
            + '<div class="actions"><a class="btn" href="/raw/' + encodePath(data.path) + '">Download</a></div></div>'
            + '<div class="empty">Binary file is not inlined.</div></div>';
          return;
        }

        const actions = '<div class="actions">'
          + '<a class="btn" href="/raw/' + encodePath(data.path) + '">Raw</a>'
          + '<button type="button" class="btn" id="copy-path">Copy path</button>'
          + "</div>";

        if (data.language === "markdown") {
          const html = renderMarkdown(data.content);
          content.innerHTML = renderCrumbs(data.path)
            + '<div class="file-box"><div class="file-box-head"><div class="title">'
            + '<span class="dir-ico">' + ICO.file + "</span>"
            + '<span class="name">' + escapeHtml(data.name) + "</span></div>"
            + '<div class="meta">markdown · ' + data.size + " bytes</div>"
            + actions + "</div>"
            + '<article class="md" id="md-view">' + html + "</article></div>";
          highlightBlocks(content);
          const nodes = content.querySelectorAll(".mermaid");
          if (nodes.length) {
            await mermaid.run({ nodes });
          }
        } else {
          const lang = data.language || "plaintext";
          content.innerHTML = renderCrumbs(data.path)
            + '<div class="file-box"><div class="file-box-head"><div class="title">'
            + '<span class="dir-ico">' + ICO.file + "</span>"
            + '<span class="name">' + escapeHtml(data.name) + "</span></div>"
            + '<div class="meta">' + escapeHtml(lang) + " · " + data.size + " bytes</div>"
            + actions + "</div>"
            + renderCodeView(data.content, lang) + "</div>";
        }

        const copyPath = $("copy-path");
        if (copyPath) {
          copyPath.addEventListener("click", async () => {
            try {
              await navigator.clipboard.writeText(data.path);
              toast("Path copied");
            } catch {
              toast(data.path);
            }
          });
        }
      } catch (err) {
        const msg = err.message || String(err);
        if (opts.deleted && msg === "not found") {
          state.currentPath = path;
          state.currentType = "deleted";
          setActiveTree(path);
          content.innerHTML = renderCrumbs(path) + '<div class="empty">File deleted</div>';
        } else {
          content.innerHTML = '<div class="err">' + escapeHtml(msg) + "</div>";
        }
      } finally {
        if (opts.preserveScroll) {
          requestAnimationFrame(() => {
            const nextMax = Math.max(document.documentElement.scrollHeight - window.innerHeight, 0);
            window.scrollTo(0, nextMax * scrollRatio);
          });
        }
      }
    }

    const TYPE_ALIASES = {
      md: [".md", ".markdown"], markdown: [".md", ".markdown"],
      kt: [".kt", ".kts"], kotlin: [".kt", ".kts"],
      ts: [".ts", ".tsx"], js: [".js", ".jsx", ".mjs", ".cjs"],
      py: [".py"], go: [".go"], rs: [".rs"], java: [".java"],
      json: [".json"], yml: [".yml", ".yaml"], yaml: [".yml", ".yaml"],
    };

    function parseSearchQuery(raw) {
      const terms = [];
      const exts = [];
      for (const token of raw.trim().split(/\\s+/).filter(Boolean)) {
        const lower = token.toLowerCase();
        if (lower.startsWith("ext:") || lower.startsWith("type:")) {
          const val = lower.slice(lower.indexOf(":") + 1).replace(/^\\./, "");
          if (!val) continue;
          const mapped = TYPE_ALIASES[val];
          if (mapped) mapped.forEach((e) => { if (!exts.includes(e)) exts.push(e); });
          else if (!exts.includes("." + val)) exts.push("." + val);
          continue;
        }
        if (lower.startsWith(".") && lower.length > 1 && !lower.slice(1).includes("/")) {
          if (!exts.includes(lower)) exts.push(lower);
          continue;
        }
        terms.push(token);
      }
      return { terms, exts };
    }

    function basenameOf(path) {
      const i = path.lastIndexOf("/");
      return i >= 0 ? path.slice(i + 1) : path;
    }

    function extOf(path) {
      const base = basenameOf(path).toLowerCase();
      const dot = base.lastIndexOf(".");
      return dot > 0 ? base.slice(dot) : "";
    }

    function parentOf(path) {
      const i = path.lastIndexOf("/");
      return i >= 0 ? path.slice(0, i) : "";
    }

    // fzf 风格: 连续命中加分, 间隙过大直接丢弃, 避免 readme 命中 dt-metadata.
    function fuzzyScore(text, query) {
      if (!query) return 0;
      const hay = text.toLowerCase();
      const needle = query.toLowerCase();
      if (hay === needle) return 10000;
      const exact = hay.indexOf(needle);
      if (exact >= 0) {
        let score = 5000 - exact * 2 - Math.max(0, hay.length - needle.length);
        if (exact === 0) score += 800;
        else {
          const prev = hay[exact - 1];
          if (prev === "/" || prev === "-" || prev === "_" || prev === ".") score += 400;
        }
        return score;
      }
      let qi = 0, score = 0, prevMatch = -2, gaps = 0;
      for (let i = 0; i < hay.length && qi < needle.length; i++) {
        if (hay[i] !== needle[qi]) continue;
        let gain = 12;
        if (i === prevMatch + 1) gain += 48;
        else if (prevMatch >= 0) {
          gaps += i - prevMatch - 1;
          gain -= Math.min(30, (i - prevMatch - 1) * 4);
        }
        if (i === 0) gain += 24;
        else {
          const prev = hay[i - 1];
          if (prev === "/" || prev === "-" || prev === "_" || prev === ".") gain += 20;
        }
        score += gain;
        prevMatch = i;
        qi += 1;
      }
      if (qi < needle.length) return -1;
      if (gaps > Math.max(4, needle.length)) return -1;
      score -= gaps * 6 + Math.max(0, hay.length - needle.length);
      return score > 0 ? score : -1;
    }

    function scorePath(path, query) {
      if (query.exts.length) {
        if (!query.exts.includes(extOf(path))) return -1;
      }
      if (!query.terms.length) return query.exts.length ? 1 : 0;
      const base = basenameOf(path);
      let total = 0;
      for (const term of query.terms) {
        const baseScore = fuzzyScore(base, term);
        const pathScore = fuzzyScore(path, term);
        const fileLike = term.includes(".");
        if (fileLike && baseScore < 0) return -1;
        const best = Math.max(baseScore >= 0 ? baseScore + 1200 : -1, fileLike ? -1 : pathScore);
        if (best < 0) return -1;
        total += best;
      }
      total -= Math.min(80, path.split("/").length * 4);
      return total;
    }

    function rankPaths(files, rawQuery, recent, limit) {
      limit = limit || 50;
      const query = parseSearchQuery(rawQuery);
      const empty = !query.terms.length && !query.exts.length;
      if (empty) {
        const seen = new Set();
        const out = [];
        for (const path of recent) {
          if (!files.includes(path) || seen.has(path)) continue;
          seen.add(path);
          out.push({ path, score: 1000000 - out.length, recent: true });
          if (out.length >= limit) return out;
        }
        for (const path of files) {
          if (seen.has(path)) continue;
          out.push({ path, score: 0, recent: false });
          if (out.length >= limit) break;
        }
        return out;
      }
      const boost = new Map();
      recent.forEach((path, idx) => boost.set(path, Math.max(0, 200 - idx * 8)));
      const ranked = [];
      for (const path of files) {
        const base = scorePath(path, query);
        if (base < 0) continue;
        ranked.push({ path, score: base + (boost.get(path) || 0), recent: boost.has(path) });
      }
      ranked.sort((a, b) => b.score - a.score || a.path.localeCompare(b.path));
      return ranked.slice(0, limit);
    }

    function openPalette() {
      state.paletteOpen = true;
      $("palette").classList.add("open");
      $("palette").setAttribute("aria-hidden", "false");
      $("palette-input").value = "";
      renderPalette("");
      $("palette-input").focus();
    }

    function closePalette() {
      state.paletteOpen = false;
      $("palette").classList.remove("open");
      $("palette").setAttribute("aria-hidden", "true");
    }

    function renderPalette(query) {
      const ranked = rankPaths(state.files, query || "", loadRecent(), 50);
      state.paletteItems = ranked.map((x) => x.path);
      state.paletteMeta = ranked;
      state.paletteIndex = 0;
      state.paletteMode = (!query || !query.trim()) ? "recent" : "search";
      paintPalette();
    }

    function paintPalette() {
      const box = $("palette-results");
      const meta = state.paletteMeta || [];
      if (!meta.length) {
        box.innerHTML = '<div class="palette-empty">No matching files</div>';
        return;
      }
      const emptyMode = state.paletteMode === "recent";
      let html = "";
      let paintedRecentHead = false;
      let paintedFilesHead = false;
      meta.forEach((item, idx) => {
        if (emptyMode && item.recent && !paintedRecentHead) {
          html += '<div class="palette-section">Recent</div>';
          paintedRecentHead = true;
        } else if (emptyMode && !item.recent && !paintedFilesHead) {
          html += '<div class="palette-section">Files</div>';
          paintedFilesHead = true;
        }
        const cls = idx === state.paletteIndex ? " active" : "";
        const base = basenameOf(item.path);
        const parent = parentOf(item.path);
        const ext = extOf(item.path).replace(/^\\./, "") || "file";
        html += '<button type="button" class="palette-item' + cls + '" data-idx="' + idx + '">'
          + '<span class="dir-ico">' + ICO.file + '</span>'
          + '<span class="meta"><span class="name">' + escapeHtml(base) + '</span>'
          + (parent ? '<span class="sub">' + escapeHtml(parent) + '</span>' : '')
          + '</span><span class="ext">' + escapeHtml(ext) + '</span></button>';
      });
      box.innerHTML = html;
      for (const btn of box.querySelectorAll(".palette-item")) {
        btn.addEventListener("click", () => {
          navigate(state.paletteItems[Number(btn.dataset.idx)] || "");
          closePalette();
        });
      }
      const active = box.querySelector(".palette-item.active");
      if (active) active.scrollIntoView({ block: "nearest" });
    }

    function navigate(path) {
      pushRecent(path);
      const url = "/" + encodePath(path);
      history.pushState({}, "", url);
      renderBlob(path);
    }

    async function refreshTreeDirs(dirs) {
      const targets = [...new Set(dirs)];
      for (const dir of targets) {
        delete state.treeKids[dir];
      }
      for (const dir of targets) {
        if (dir !== "" && !state.expanded[dir]) continue;
        try {
          await ensureTree(dir);
        } catch {
          delete state.treeKids[dir];
          delete state.expanded[dir];
        }
      }
      state.rootEntries = state.treeKids[""] || [];
      paintTree();
      setActiveTree(state.currentPath);
    }

    async function refreshIndex() {
      const idx = await api("/api/index");
      state.files = idx.files || [];
      if (state.paletteOpen) {
        renderPalette($("palette-input").value || "");
      }
    }

    async function resyncWatch() {
      await refreshIndex();
      const dirs = ["", ...Object.keys(state.expanded)];
      await refreshTreeDirs(dirs);
      await renderBlob(currentRelPath(), {
        preserveScroll: true,
        deleted: true,
      });
    }

    function currentAffected(change) {
      const path = state.currentPath;
      if (change.path === path || path.startsWith(change.path + "/")) {
        return true;
      }
      return state.currentType === "dir" && parentOf(change.path) === path;
    }

    async function applyWatchEvent(event) {
      if (event.type === "resync") {
        await resyncWatch();
        return;
      }
      if (event.type !== "files-changed" || !Array.isArray(event.changes)) {
        return;
      }

      const topology = event.changes.some((change) => change.kind !== "change");
      if (topology) {
        await refreshIndex();
      }

      const dirs = new Set();
      let affected = false;
      let deleted = false;
      for (const change of event.changes) {
        affected ||= currentAffected(change);
        deleted ||= change.kind === "delete"
          && (state.currentPath === change.path || state.currentPath.startsWith(change.path + "/"));
        if (change.kind === "change") continue;
        dirs.add(parentOf(change.path));
        if (change.kind === "delete") {
          for (const dir of Object.keys(state.treeKids)) {
            if (dir === change.path || dir.startsWith(change.path + "/")) {
              delete state.treeKids[dir];
              delete state.expanded[dir];
            }
          }
        }
      }
      if (dirs.size > 0) {
        await refreshTreeDirs([...dirs]);
      }
      if (affected) {
        await renderBlob(currentRelPath(), {
          preserveScroll: true,
          deleted,
        });
      }
    }

    function enqueueWatch(event) {
      state.watchQueue = state.watchQueue
        .then(() => applyWatchEvent(event))
        .catch((err) => toast(err.message || String(err)));
    }

    function scheduleWatchReconnect() {
      const delays = [500, 1000, 2000, 5000];
      const idx = Math.min(state.watchRetry, delays.length - 1);
      clearTimeout(state.watchTimer);
      state.watchTimer = setTimeout(connectWatch, delays[idx]);
      state.watchRetry += 1;
    }

    function connectWatch() {
      if (state.watchStatus !== "active") return;
      paintWatchStatus("connecting");
      const scheme = location.protocol === "https:" ? "wss:" : "ws:";
      const ws = new WebSocket(scheme + "//" + location.host + "/api/watch");
      state.watchSocket = ws;
      ws.addEventListener("open", () => {
        const reconnect = state.watchSeen;
        state.watchSeen = true;
        state.watchRetry = 0;
        paintWatchStatus("active");
        if (reconnect) enqueueWatch({ type: "resync" });
      });
      ws.addEventListener("message", (msg) => {
        try {
          const event = JSON.parse(String(msg.data));
          if (event.type === "watch-status" && event.status === "unavailable") {
            state.watchStatus = "unavailable";
            paintWatchStatus("unavailable");
            ws.close();
            return;
          }
          enqueueWatch(event);
        } catch {
          toast("Invalid watch event");
        }
      });
      ws.addEventListener("error", () => ws.close());
      ws.addEventListener("close", () => {
        if (state.watchSocket === ws) state.watchSocket = null;
        if (state.watchStatus !== "active") return;
        paintWatchStatus("disconnected");
        scheduleWatchReconnect();
      });
    }

    async function boot() {
      // 先拿启动期投影再连 watch, 避免首屏与变更事件交叉。
      try {
        const [idx, meta, root] = await Promise.all([
          api("/api/index"),
          api("/api/meta"),
          api("/api/blob?path="),
        ]);
        state.files = idx.files || [];
        state.watchStatus = meta.watch;
        paintWatchStatus(state.watchStatus);
        if (root.type === "dir") {
          state.rootEntries = root.entries || [];
          state.treeKids[""] = state.rootEntries;
        }
      } catch (err) {
        $("tree").innerHTML = '<div class="err">' + escapeHtml(err.message || String(err)) + "</div>";
      }
      // renderBlob 内 revealTree 会按深链展开侧栏.
      await renderBlob(currentRelPath());
      if (state.watchStatus === "active") {
        connectWatch();
      }
    }

    $("btn-open").addEventListener("click", openPalette);
    $("btn-copy").addEventListener("click", async () => {
      try {
        await navigator.clipboard.writeText(location.href);
        toast("URL copied");
      } catch {
        prompt("Copy URL", location.href);
      }
    });
    $("palette").addEventListener("click", (e) => {
      if (e.target === $("palette")) closePalette();
    });
    $("palette-input").addEventListener("input", (e) => {
      renderPalette(e.target.value || "");
    });

    window.addEventListener("keydown", (e) => {
      const isPalette = (e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k";
      if (isPalette) {
        e.preventDefault();
        if (state.paletteOpen) closePalette();
        else openPalette();
        return;
      }
      if (!state.paletteOpen) return;
      if (e.key === "Escape") {
        e.preventDefault();
        closePalette();
        return;
      }
      if (e.key === "ArrowDown") {
        e.preventDefault();
        state.paletteIndex = Math.min(
          state.paletteIndex + 1,
          Math.max(state.paletteItems.length - 1, 0),
        );
        paintPalette();
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        state.paletteIndex = Math.max(state.paletteIndex - 1, 0);
        paintPalette();
        return;
      }
      if (e.key === "Enter") {
        e.preventDefault();
        const path = state.paletteItems[state.paletteIndex];
        if (path) {
          navigate(path);
          closePalette();
        }
      }
    });

    window.addEventListener("popstate", () => {
      renderBlob(currentRelPath());
    });

    document.addEventListener("click", async (e) => {
      const toggle = e.target.closest("[data-tree-toggle]");
      if (toggle) {
        e.preventDefault();
        e.stopPropagation();
        const dir = toggle.getAttribute("data-tree-toggle") || "";
        if (state.expanded[dir]) {
          delete state.expanded[dir];
        } else {
          state.expanded[dir] = true;
          try {
            await ensureTree(dir);
          } catch (err) {
            toast(err.message || String(err));
          }
        }
        paintTree();
        setActiveTree(state.currentPath);
        return;
      }
      const a = e.target.closest("a");
      if (!a) return;
      const href = a.getAttribute("href") || "";
      if (!href.startsWith("/") || href.startsWith("//") || href.startsWith("/raw/") || href.startsWith("/api/")) {
        return;
      }
      e.preventDefault();
      const path = decodeURIComponent(href.replace(/^\\/+/, ""));
      navigate(path);
    });

    boot();
`;
}
