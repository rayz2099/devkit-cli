/**
 * 浏览器端富预览。表格库走 CDN, 是为了不把 SheetJS/mammoth 打进单二进制。
 */
export const CLIENT_PREVIEW = String.raw`
    function loadScript(url) {
      if (state.scripts[url]) return state.scripts[url];
      state.scripts[url] = new Promise((resolve, reject) => {
        const el = document.createElement("script");
        el.src = url;
        el.async = true;
        el.onload = () => resolve();
        el.onerror = () => reject(new Error("脚本加载失败"));
        document.head.appendChild(el);
      });
      return state.scripts[url];
    }

    function fileBoxOpen(data, meta, actions) {
      return renderCrumbs(data.path)
        + '<div class="file-box"><div class="file-box-head"><div class="title">'
        + '<span class="dir-ico">' + ICO.file + "</span>"
        + '<span class="name">' + escapeHtml(data.name) + "</span></div>"
        + '<div class="meta">' + meta + "</div>"
        + '<div class="actions">' + actions + "</div></div>";
    }

    function bindCopy(path) {
      const copyPath = $("copy-path");
      if (!copyPath) return;
      copyPath.addEventListener("click", async () => {
        try {
          await navigator.clipboard.writeText(path);
          toast("Path copied");
        } catch (err) {
          toast(path);
        }
      });
    }

    function copyPathBtn() {
      return '<button type="button" class="btn" id="copy-path">Copy path</button>';
    }

    function isRichKind(kind) {
      return kind === "csv" || kind === "tsv" || kind === "jsonl" || kind === "xlsx"
        || kind === "pdf" || kind === "docx" || kind === "video" || kind === "audio" || kind === "html";
    }

    function previewActions(data) {
      const kind = data.preview || "";
      const raw = '<a class="btn" href="/raw/' + encodePath(data.path) + '">Raw</a>';
      if (kind === "html") {
        return '<button type="button" class="btn active" data-pv-mode="preview">Preview</button>'
          + '<button type="button" class="btn" data-pv-mode="code">Code</button>'
          + '<a class="btn" target="_blank" rel="noreferrer" href="/raw/' + encodePath(data.path) + '">New Tab</a>'
          + copyPathBtn();
      }
      if (kind === "csv" || kind === "tsv" || kind === "jsonl") {
        return '<button type="button" class="btn active" data-pv-mode="table">Table</button>'
          + '<button type="button" class="btn" data-pv-mode="code">Code</button>'
          + raw + copyPathBtn();
      }
      return raw + copyPathBtn();
    }

    const PREVIEW_PAGE = 1000;
    const SHEET_JS = "https://cdn.jsdelivr.net/npm/xlsx@0.18.5/dist/xlsx.full.min.js";
    const MAMMOTH_JS = "https://cdn.jsdelivr.net/npm/mammoth@1.8.0/mammoth.browser.min.js";

    function parseDelimited(text, sep) {
      const src = text.charCodeAt(0) === 0xfeff ? text.slice(1) : text;
      const rows = [];
      let row = [];
      let cell = "";
      let quote = false;
      for (let i = 0; i < src.length; i += 1) {
        const ch = src[i];
        if (quote) {
          if (ch === '"') {
            if (src[i + 1] === '"') {
              cell += '"';
              i += 1;
            } else {
              quote = false;
            }
          } else {
            cell += ch;
          }
          continue;
        }
        if (ch === '"') {
          quote = true;
          continue;
        }
        if (ch === sep) {
          row.push(cell);
          cell = "";
          continue;
        }
        if (ch === "\n") {
          row.push(cell);
          rows.push(row);
          row = [];
          cell = "";
          continue;
        }
        if (ch === "\r") continue;
        cell += ch;
      }
      if (cell.length || row.length) {
        row.push(cell);
        rows.push(row);
      }
      if (!rows.length) return { header: [], body: [] };
      let width = 0;
      for (const item of rows) width = Math.max(width, item.length);
      const norm = rows.map((item) => {
        const copy = item.slice();
        while (copy.length < width) copy.push("");
        return copy;
      });
      const body = norm.slice(1).map((item, idx) => ({ n: idx + 2, cells: item }));
      return { header: norm[0], body: body };
    }

    function cellText(value) {
      if (value === undefined || value === null) return "";
      if (typeof value === "object") return JSON.stringify(value);
      return String(value);
    }

    function parseJsonl(text) {
      const keys = [];
      const records = [];
      const lines = String(text).split("\n");
      for (const line of lines) {
        if (!line.trim()) continue;
        try {
          const val = JSON.parse(line);
          if (val && typeof val === "object" && !Array.isArray(val)) {
            for (const key of Object.keys(val)) {
              if (!keys.includes(key)) keys.push(key);
            }
            records.push(val);
          } else {
            if (!keys.includes("_value")) keys.push("_value");
            records.push({ _value: val });
          }
        } catch (err) {
          if (!keys.includes("_raw")) keys.push("_raw");
          records.push({ _raw: line });
        }
      }
      const body = records.map((rec, idx) => ({
        n: idx + 1,
        cells: keys.map((key) => cellText(rec[key])),
      }));
      return { header: keys, body: body };
    }

    async function fetchRawText(path) {
      const res = await fetch("/raw/" + encodePath(path));
      if (!res.ok) throw new Error("raw " + res.status);
      return res.text();
    }

    async function fetchRawBuf(path) {
      const res = await fetch("/raw/" + encodePath(path));
      if (!res.ok) throw new Error("raw " + res.status);
      return res.arrayBuffer();
    }

    function compareCells(a, b) {
      return String(a ?? "").localeCompare(String(b ?? ""), undefined, { numeric: true, sensitivity: "base" });
    }

    function tableHtml(header, rows, sortCol, sortDir) {
      let head = '<tr><th class="ln"></th>';
      header.forEach((name, i) => {
        const mark = sortCol === i ? (sortDir > 0 ? " ↑" : " ↓") : "";
        head += '<th data-col="' + i + '">' + escapeHtml(name) + mark + "</th>";
      });
      head += "</tr>";
      let body = "";
      for (const row of rows) {
        body += '<tr><td class="ln">' + row.n + "</td>";
        for (let i = 0; i < header.length; i += 1) {
          body += "<td>" + escapeHtml(row.cells[i] ?? "") + "</td>";
        }
        body += "</tr>";
      }
      return '<table class="grid"><thead>' + head + "</thead><tbody>" + body + "</tbody></table>";
    }

    // 合并单元格绑在原始行列上。筛选或排序会打乱几何, 这时改画普通网格。
    function mountGrid(host, model) {
      host.innerHTML = '<div class="preview-bar"><input class="grid-filter" placeholder="过滤行" />'
        + '<span class="grid-count"></span></div><div class="grid-wrap"></div><div class="grid-foot"></div>';
      const input = host.querySelector(".grid-filter");
      const wrap = host.querySelector(".grid-wrap");
      const foot = host.querySelector(".grid-foot");
      const count = host.querySelector(".grid-count");
      let query = "";
      let sortCol = -1;
      let sortDir = 1;
      let shown = PREVIEW_PAGE;

      function rowsNow() {
        let rows = model.body;
        if (query) {
          const q = query.toLowerCase();
          rows = rows.filter((row) => row.cells.some((cell) => String(cell).toLowerCase().includes(q)));
        }
        if (sortCol >= 0) {
          const col = sortCol;
          const dir = sortDir;
          rows = rows.slice().sort((a, b) => compareCells(a.cells[col], b.cells[col]) * dir);
        }
        return rows;
      }

      function paint() {
        const rows = rowsNow();
        const slice = rows.slice(0, shown);
        const useMerge = model.merges && model.merges.length && !query && sortCol < 0;
        wrap.innerHTML = useMerge
          ? mergeTable(model, shown)
          : tableHtml(model.header, slice, sortCol, sortDir);
        count.textContent = slice.length + " / " + rows.length;
        foot.innerHTML = rows.length > shown
          ? '<button type="button" class="btn" data-more>加载更多</button><button type="button" class="btn" data-all>全部</button>'
          : "";
        wrap.querySelectorAll("th[data-col]").forEach((th) => {
          th.addEventListener("click", () => {
            const col = Number(th.getAttribute("data-col"));
            if (sortCol === col) sortDir = -sortDir;
            else {
              sortCol = col;
              sortDir = 1;
            }
            shown = PREVIEW_PAGE;
            paint();
          });
        });
        const more = foot.querySelector("[data-more]");
        if (more) more.addEventListener("click", () => { shown += PREVIEW_PAGE; paint(); });
        const all = foot.querySelector("[data-all]");
        if (all) all.addEventListener("click", () => { shown = rows.length; paint(); });
      }

      input.addEventListener("input", () => {
        query = input.value.trim();
        shown = PREVIEW_PAGE;
        paint();
      });
      paint();
    }

    function sheetMatrix(sheet) {
      const matrix = XLSX.utils.sheet_to_json(sheet, { header: 1, raw: false, defval: "" });
      let width = 0;
      for (const row of matrix) width = Math.max(width, row.length);
      const norm = matrix.map((row) => {
        const copy = row.map((cell) => cellText(cell));
        while (copy.length < width) copy.push("");
        return copy;
      });
      if (!norm.length) return { header: [], body: [] };
      const body = norm.slice(1).map((cells, idx) => ({ n: idx + 2, cells: cells }));
      return { header: norm[0], body: body, merges: sheet["!merges"] || [] };
    }

    function indexMerges(merges, rowCount) {
      const skip = new Set();
      const span = new Map();
      for (const merge of merges) {
        if (!merge || !merge.s || !merge.e) continue;
        if (merge.e.r >= rowCount) continue;
        const key = merge.s.r + "," + merge.s.c;
        span.set(key, { rs: merge.e.r - merge.s.r + 1, cs: merge.e.c - merge.s.c + 1 });
        for (let r = merge.s.r; r <= merge.e.r; r += 1) {
          for (let c = merge.s.c; c <= merge.e.c; c += 1) {
            if (r === merge.s.r && c === merge.s.c) continue;
            skip.add(r + "," + c);
          }
        }
      }
      return { skip: skip, span: span };
    }

    function mergeTable(model, shown) {
      const visible = model.body.slice(0, shown);
      const rowCount = 1 + visible.length;
      const indexed = indexMerges(model.merges || [], rowCount);
      const header = model.header;
      let html = '<tr><th class="ln">1</th>';
      for (let c = 0; c < header.length; c += 1) {
        if (indexed.skip.has("0," + c)) continue;
        const box = indexed.span.get("0," + c);
        const rs = box ? ' rowspan="' + box.rs + '"' : "";
        const cs = box ? ' colspan="' + box.cs + '"' : "";
        html += '<th data-col="' + c + '"' + rs + cs + ">" + escapeHtml(header[c] ?? "") + "</th>";
      }
      html += "</tr>";
      visible.forEach((row, idx) => {
        const r = idx + 1;
        html += "<tr><td class=\"ln\">" + row.n + "</td>";
        for (let c = 0; c < header.length; c += 1) {
          if (indexed.skip.has(r + "," + c)) continue;
          const box = indexed.span.get(r + "," + c);
          const rs = box ? ' rowspan="' + box.rs + '"' : "";
          const cs = box ? ' colspan="' + box.cs + '"' : "";
          html += "<td" + rs + cs + ">" + escapeHtml(row.cells[c] ?? "") + "</td>";
        }
        html += "</tr>";
      });
      return '<table class="grid"><tbody>' + html + "</tbody></table>";
    }

    async function mountExcel(host, path) {
      await loadScript(SHEET_JS);
      if (!window.XLSX) throw new Error("SheetJS 未加载");
      const buf = await fetchRawBuf(path);
      const wb = XLSX.read(buf, { type: "array" });
      const names = wb.SheetNames || [];
      if (!names.length) {
        host.innerHTML = '<div class="empty">空工作簿</div>';
        return;
      }
      let name = names[0];
      host.innerHTML = '<div class="sheet-tabs"></div><div class="grid-host"></div>';
      const tabs = host.querySelector(".sheet-tabs");
      const gridHost = host.querySelector(".grid-host");
      const show = () => {
        tabs.innerHTML = names.map((item) => {
          const cls = item === name ? "btn active" : "btn";
          return '<button type="button" class="' + cls + '" data-sheet="' + escapeHtml(item) + '">' + escapeHtml(item) + "</button>";
        }).join("");
        tabs.querySelectorAll("[data-sheet]").forEach((btn) => {
          btn.addEventListener("click", () => {
            name = btn.getAttribute("data-sheet") || name;
            show();
          });
        });
        mountGrid(gridHost, sheetMatrix(wb.Sheets[name]));
      };
      show();
    }

    async function mountDocx(host, path) {
      await loadScript(MAMMOTH_JS);
      if (!window.mammoth) throw new Error("mammoth 未加载");
      const buf = await fetchRawBuf(path);
      const result = await mammoth.convertToHtml({ arrayBuffer: buf });
      host.innerHTML = '<article class="docx md">' + (result.value || "") + "</article>";
    }

    async function renderRichPreview(data) {
      const host = $("pv-root");
      if (!host) return;
      const kind = data.preview;
      let mode = kind === "csv" || kind === "tsv" || kind === "jsonl" ? "table" : "preview";

      async function paint() {
        host.innerHTML = '<div class="empty">Loading…</div>';
        try {
          if (mode === "code") {
            const text = data.content !== undefined ? data.content : await fetchRawText(data.path);
            await mountCode(host, text, data.language || "plaintext", data.path);
            return;
          }
          if (kind === "csv" || kind === "tsv") {
            const text = data.content !== undefined ? data.content : await fetchRawText(data.path);
            const sep = kind === "tsv" ? "\t" : ",";
            mountGrid(host, parseDelimited(text, sep));
            return;
          }
          if (kind === "jsonl") {
            const text = data.content !== undefined ? data.content : await fetchRawText(data.path);
            mountGrid(host, parseJsonl(text));
            return;
          }
          if (kind === "xlsx") {
            await mountExcel(host, data.path);
            return;
          }
          if (kind === "docx") {
            await mountDocx(host, data.path);
            return;
          }
          if (kind === "pdf") {
            host.innerHTML = '<iframe class="pdf-frame" title="pdf" src="/raw/' + encodePath(data.path) + '"></iframe>';
            return;
          }
          if (kind === "video") {
            host.innerHTML = '<div class="media-wrap"><video controls src="/raw/' + encodePath(data.path) + '"></video></div>';
            return;
          }
          if (kind === "audio") {
            host.innerHTML = '<div class="media-wrap"><audio controls src="/raw/' + encodePath(data.path) + '"></audio></div>';
            return;
          }
          if (kind === "html") {
            // 同源才能跑相对 module 和报表脚本; 这是本地只读预览, 文件来自当前工作区。
            host.innerHTML = '<iframe class="html-frame" title="html" sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-modals" src="/raw/' + encodePath(data.path) + '"></iframe>';
          }
        } catch (err) {
          host.innerHTML = '<div class="err">' + escapeHtml(err.message || String(err)) + "</div>";
        }
      }

      document.querySelectorAll("[data-pv-mode]").forEach((btn) => {
        btn.addEventListener("click", () => {
          mode = btn.getAttribute("data-pv-mode") || mode;
          document.querySelectorAll("[data-pv-mode]").forEach((item) => {
            item.classList.toggle("active", item === btn);
          });
          paint();
        });
      });
      await paint();
    }
`;
