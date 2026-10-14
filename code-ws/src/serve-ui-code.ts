/**
 * 浏览器端代码视图。
 * 整文件高亮是为了保住跨行注释; import 用纯文本包链接, 是因为高亮 span 会把路径切碎, 锚点不能跨 span。
 */
export const CLIENT_CODE = String.raw`
    function hasHljs() {
      return typeof hljs !== "undefined" && hljs && typeof hljs.highlight === "function";
    }

    function hljsName(lang) {
      if (lang === "tsx") return "typescript";
      if (lang === "jsx") return "javascript";
      if (!lang || lang === "plaintext") return "";
      return lang;
    }

    // 语言包失败就纯文本。逐行自动识别会把跨行 token 切碎后染错。
    async function ensureHljsLang(name) {
      if (!name || !hasHljs()) return false;
      if (hljs.getLanguage(name)) return true;
      if (state.hljsMiss[name]) return false;
      const url = "https://cdn.jsdelivr.net/gh/highlightjs/cdn-release@11.11.1/build/languages/" + name + ".min.js";
      try {
        await loadScript(url);
      } catch (err) {
        state.hljsMiss[name] = true;
        return false;
      }
      if (hljs.getLanguage(name)) return true;
      state.hljsMiss[name] = true;
      return false;
    }

    function splitHlLines(html) {
      const lines = String(html).split("\n");
      const open = [];
      return lines.map((line) => {
        const start = open.join("");
        const re = /<\/span>|<span\b[^>]*>/g;
        let m;
        while ((m = re.exec(line))) {
          if (m[0] === "</span>") open.pop();
          else open.push(m[0]);
        }
        return start + line + "</span>".repeat(open.length);
      });
    }

    async function highlightBlocks(root) {
      if (!hasHljs() || !root) return;
      const blocks = root.querySelectorAll("pre code");
      for (const block of blocks) {
        const cls = block.className || "";
        const found = cls.match(/language-([\w#+.-]+)/);
        const name = hljsName(found ? found[1] : "");
        if (!name) continue;
        const ok = await ensureHljsLang(name);
        if (!ok) continue;
        const src = block.textContent || "";
        block.classList.add("hljs");
        block.innerHTML = hljs.highlight(src, { language: name }).value;
      }
    }

    function langFamily(lang) {
      if (lang === "typescript" || lang === "tsx" || lang === "javascript" || lang === "jsx") return "js";
      if (lang === "kotlin" || lang === "java") return "jvm";
      if (lang === "go") return "go";
      if (lang === "python") return "py";
      if (lang === "c" || lang === "cpp") return "c";
      return "";
    }

    function specsOnLine(line, lang) {
      const family = langFamily(lang);
      if (family === "js") return jsSpecs(line);
      if (family === "jvm") return jvmSpecs(line);
      if (family === "go") return goSpecs(line);
      if (family === "py") return pySpecs(line);
      if (family === "c") return cSpecs(line);
      return [];
    }

    function jsSpecs(line) {
      const out = [];
      const re = /['"](\.[^'"]+)['"]/g;
      let m;
      while ((m = re.exec(line))) {
        const spec = m[1];
        const start = m.index + 1;
        const pre = line.slice(Math.max(0, m.index - 30), m.index);
        if (!/(?:from|import|require\(|import\()\s*$/.test(pre)) continue;
        out.push({ start: start, end: start + spec.length, spec: spec });
      }
      return out;
    }

    function jvmSpecs(line) {
      const m = line.match(/\bimport\s+(?:static\s+)?([A-Za-z_][\w.]*)/);
      if (!m) return [];
      const spec = m[1];
      const start = m.index + m[0].length - spec.length;
      return [{ start: start, end: start + spec.length, spec: spec }];
    }

    function goSpecs(line) {
      if (!/^\s*import\b/.test(line) && !/^\s*(?:[A-Za-z0-9_.]+\s+)?"[^"]+"\s*$/.test(line)) return [];
      const out = [];
      const re = /"([^"]+)"/g;
      let m;
      while ((m = re.exec(line))) {
        const spec = m[1];
        if (!spec || spec.indexOf(" ") >= 0) continue;
        const start = m.index + 1;
        out.push({ start: start, end: start + spec.length, spec: spec });
      }
      return out;
    }

    function pySpecs(line) {
      let m = line.match(/^\s*from\s+([A-Za-z_][\w.]*)\s+import\b/);
      if (!m) m = line.match(/^\s*import\s+([A-Za-z_][\w.]*)/);
      if (!m) return [];
      const spec = m[1];
      const start = line.indexOf(spec);
      return [{ start: start, end: start + spec.length, spec: spec }];
    }

    function cSpecs(line) {
      const m = line.match(/^\s*#\s*include\s+[<"]([^>"]+)[>"]/);
      if (!m) return [];
      const spec = m[1];
      const start = line.indexOf(spec);
      return [{ start: start, end: start + spec.length, spec: spec }];
    }

    function fileSet() {
      if (!state.fileSet || state.fileSetSize !== state.files.length) {
        state.fileSet = new Set(state.files);
        state.fileSetSize = state.files.length;
      }
      return state.fileSet;
    }

    function joinPosix(dir, spec) {
      const raw = (dir ? dir + "/" + spec : spec).split("/");
      const parts = [];
      for (const p of raw) {
        if (!p || p === ".") continue;
        if (p === "..") parts.pop();
        else parts.push(p);
      }
      return parts.join("/");
    }

    function uniqueSuffix(suffix) {
      let hit = "";
      for (const f of state.files) {
        if (f !== suffix && !f.endsWith("/" + suffix)) continue;
        if (hit) return "";
        hit = f;
      }
      return hit;
    }

    function resolveRel(fromFile, spec) {
      if (!spec.startsWith(".")) return "";
      const joined = joinPosix(parentOf(fromFile), spec);
      const set = fileSet();
      if (set.has(joined)) return joined;
      const exts = [".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".json", ".css"];
      const hits = [];
      for (const ext of exts) {
        const file = joined + ext;
        const index = joined + "/index" + ext;
        if (set.has(file)) hits.push(file);
        if (set.has(index)) hits.push(index);
      }
      return hits.length === 1 ? hits[0] : "";
    }

    function resolveDotted(spec, exts) {
      const parts = spec.split(".");
      for (let i = parts.length; i >= 1; i -= 1) {
        const suffix = parts.slice(0, i).join("/");
        for (const ext of exts) {
          const hit = uniqueSuffix(suffix + ext);
          if (hit) return hit;
        }
        for (const ext of exts) {
          const hit = uniqueSuffix(suffix + "/__init__" + ext);
          if (hit) return hit;
        }
      }
      return "";
    }

    function resolveGo(spec) {
      const direct = uniqueSuffix(spec + ".go");
      if (direct) return direct;
      const hits = [];
      const needle = "/" + spec + "/";
      for (const f of state.files) {
        if (!f.endsWith(".go")) continue;
        if (f.startsWith(spec + "/") || f.indexOf(needle) >= 0) hits.push(f);
        if (hits.length > 1) return "";
      }
      return hits.length === 1 ? hits[0] : "";
    }

    function resolveSpec(fromFile, spec, lang) {
      const family = langFamily(lang);
      if (family === "js") return resolveRel(fromFile, spec);
      if (family === "jvm") return resolveDotted(spec, [".kt", ".kts", ".java"]);
      if (family === "py") return resolveDotted(spec, [".py"]);
      if (family === "go") return resolveGo(spec);
      if (family === "c") return uniqueSuffix(spec);
      return "";
    }

    function findLineLinks(line, path, lang) {
      const specs = specsOnLine(line, lang);
      const links = [];
      for (const spec of specs) {
        const target = resolveSpec(path, spec.spec, lang);
        if (!target) continue;
        links.push({ start: spec.start, end: spec.end, href: "/" + encodePath(target) });
      }
      return links;
    }

    function linkPlain(line, links) {
      const sorted = links.slice().sort((a, b) => a.start - b.start);
      let out = "";
      let i = 0;
      for (const link of sorted) {
        if (link.start < i || link.end > line.length) continue;
        out += escapeHtml(line.slice(i, link.start));
        out += '<a class="jmp" href="' + escapeHtml(link.href) + '">' + escapeHtml(line.slice(link.start, link.end)) + "</a>";
        i = link.end;
      }
      out += escapeHtml(line.slice(i));
      return out || " ";
    }

    async function renderCodeHtml(content, lang, path) {
      const src = String(content).replace(/\n$/, "");
      const rawLines = src.split("\n");
      state.codeLines = rawLines;
      state.codePath = path || "";
      const name = hljsName(lang);
      let htmlLines = rawLines.map((line) => escapeHtml(line));
      if (await ensureHljsLang(name)) {
        const highlighted = hljs.highlight(src, { language: name }).value;
        htmlLines = splitHlLines(highlighted);
      }
      const rows = rawLines.map((line, i) => {
        const links = findLineLinks(line, path, lang);
        let cell = links.length ? linkPlain(line, links) : (htmlLines[i] || "");
        if (!cell) cell = " ";
        const n = String(i + 1);
        return '<tr data-line="' + n + '"><td class="ln" data-line="' + n + '">' + n + '</td><td class="code">' + cell + "</td></tr>";
      }).join("");
      return '<div class="code-wrap"><table class="code-table"><tbody>' + rows + "</tbody></table></div>";
    }

    async function mountCode(host, content, lang, path) {
      if (!host) return;
      host.innerHTML = await renderCodeHtml(content, lang, path);
      bindCodeInteractions(host);
      applyLineSelection();
    }

    function parseLineHash() {
      const m = location.hash.match(/^#L(\d+)(?:-L(\d+))?$/i);
      if (!m) return null;
      const a = Number(m[1]);
      const b = m[2] ? Number(m[2]) : a;
      if (!a || !b) return null;
      return { start: Math.min(a, b), end: Math.max(a, b) };
    }

    function applyLineSelection() {
      const range = parseLineHash();
      document.querySelectorAll(".code-table tr").forEach((tr) => {
        const n = Number(tr.dataset.line);
        const hot = !!(range && n >= range.start && n <= range.end);
        tr.classList.toggle("line-hot", hot);
      });
      if (!range) return;
      const row = document.querySelector('.code-table tr[data-line="' + range.start + '"]');
      if (row) row.scrollIntoView({ block: "center" });
    }

    function setLineHash(hash) {
      history.replaceState({}, "", location.pathname + location.search + hash);
      applyLineSelection();
    }

    function caretPos(e) {
      if (document.caretPositionFromPoint) {
        const pos = document.caretPositionFromPoint(e.clientX, e.clientY);
        if (!pos) return null;
        return { node: pos.offsetNode, offset: pos.offset };
      }
      if (document.caretRangeFromPoint) {
        const r = document.caretRangeFromPoint(e.clientX, e.clientY);
        if (!r) return null;
        return { node: r.startContainer, offset: r.startOffset };
      }
      return null;
    }

    function wordAtEvent(e, td) {
      const tr = td.parentElement;
      const lineNo = Number(tr && tr.dataset.line);
      const line = (state.codeLines && state.codeLines[lineNo - 1]) || "";
      const pos = caretPos(e);
      if (!line || !pos || !pos.node || pos.node.nodeType !== 3) return "";
      let col = 0;
      let found = false;
      const walker = document.createTreeWalker(td, NodeFilter.SHOW_TEXT);
      while (walker.nextNode()) {
        if (walker.currentNode === pos.node) {
          col += pos.offset;
          found = true;
          break;
        }
        col += (walker.currentNode.textContent || "").length;
      }
      if (!found) return "";
      const re = /[A-Za-z_][A-Za-z0-9_]*/g;
      let m;
      while ((m = re.exec(line))) {
        const end = m.index + m[0].length;
        if (col >= m.index && col <= end) return m[0];
      }
      return "";
    }

    function closeSymPop() {
      const pop = $("sym-pop");
      if (!pop) return;
      pop.hidden = true;
      pop.innerHTML = "";
      state.symOpen = false;
    }

    function openSymPop(x, y, word, hits, truncated) {
      const pop = $("sym-pop");
      if (!pop) return;
      if (!hits.length) {
        closeSymPop();
        toast("No matches");
        return;
      }
      let html = '<div class="sym-title">' + escapeHtml(word) + "</div>";
      for (const hit of hits) {
        html += '<button type="button" class="sym-hit" data-path="' + escapeHtml(hit.path)
          + '" data-line="' + hit.line + '"><span class="k">' + hit.kind
          + '</span><span class="p">' + escapeHtml(hit.path) + ":" + hit.line
          + '</span><span class="t">' + escapeHtml(hit.text) + "</span></button>";
      }
      if (truncated) html += '<div class="sym-more">结果已截断</div>';
      pop.innerHTML = html;
      pop.hidden = false;
      state.symOpen = true;
      const left = Math.max(8, Math.min(x, window.innerWidth - 380));
      const top = Math.max(8, Math.min(y + 8, window.innerHeight - 280));
      pop.style.left = left + "px";
      pop.style.top = top + "px";
      pop.querySelectorAll(".sym-hit").forEach((btn) => {
        btn.addEventListener("click", () => {
          const path = btn.getAttribute("data-path") || "";
          const line = btn.getAttribute("data-line") || "1";
          closeSymPop();
          navigate(path, "#L" + line);
        });
      });
    }

    async function onSymbolClick(e, word) {
      let data;
      try {
        data = await api("/api/symbol?q=" + encodeURIComponent(word));
      } catch (err) {
        toast(err.message || String(err));
        return;
      }
      const hits = data.hits || [];
      const defs = hits.filter((hit) => hit.kind === "def");
      if ((e.metaKey || e.ctrlKey) && defs.length === 1) {
        closeSymPop();
        navigate(defs[0].path, "#L" + defs[0].line);
        return;
      }
      openSymPop(e.clientX, e.clientY, word, hits, !!data.truncated);
    }

    function bindCodeInteractions(host) {
      host.addEventListener("click", (e) => {
        if (e.target.closest("a")) return;
        closeSymPop();
        const ln = e.target.closest(".ln");
        if (ln) {
          e.preventDefault();
          e.stopPropagation();
          const n = Number(ln.dataset.line);
          if (!n) return;
          if (e.shiftKey && state.lineAnchor) {
            const a = Math.min(state.lineAnchor, n);
            const b = Math.max(state.lineAnchor, n);
            setLineHash("#L" + a + "-L" + b);
          } else {
            state.lineAnchor = n;
            setLineHash("#L" + n);
          }
          return;
        }
        const td = e.target.closest("td.code");
        if (!td) return;
        const sel = window.getSelection();
        if (sel && !sel.isCollapsed && td.contains(sel.anchorNode)) return;
        const word = wordAtEvent(e, td);
        if (word.length < 2) return;
        e.stopPropagation();
        onSymbolClick(e, word);
      });
    }
`;
