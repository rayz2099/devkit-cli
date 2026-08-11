import { renderServeClientScript } from "./serve-ui-client";

/**
 * 嵌入式演示 UI: GitHub Primer 风 blob/tree chrome.
 * CDN 承载 marked/hljs/mermaid, 避免把重依赖打进 bun compile 产物.
 */
export function renderServeHtml(rootName: string): string {
  const title = escapeHtml(rootName);
  const rootJson = JSON.stringify(rootName);
  return `<!doctype html>
<html lang="zh-CN" data-color-mode="light">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>${title} · code-ws serve</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/highlight.js@11.11.1/styles/github.min.css" />
  <style>
    :root {
      --bgCanvas: #ffffff;
      --bgSubtle: #f6f8fa;
      --bgInset: #f6f8fa;
      --bgOverlay: #ffffff;
      --bgNeutralMuted: rgba(175, 184, 193, 0.2);
      --bgAccentMuted: rgba(9, 105, 218, 0.1);
      --bgSuccessMuted: rgba(26, 127, 55, 0.1);
      --borderDefault: #d0d7de;
      --borderMuted: #d8dee4;
      --fgDefault: #1f2328;
      --fgMuted: #656d76;
      --fgSubtle: #8c959f;
      --fgOnEmphasis: #ffffff;
      --accentFg: #0969da;
      --accentEmphasis: #0969da;
      --successFg: #1a7f37;
      --dangerFg: #cf222e;
      --btnBg: #f6f8fa;
      --btnBorder: #d0d7de;
      --btnHoverBg: #f3f4f6;
      --btnPrimaryBg: #1f883d;
      --btnPrimaryHover: #1a7f37;
      --shadowFloat: 0 8px 24px rgba(140, 149, 159, 0.2);
      --radius: 6px;
      --radiusLg: 12px;
      --headerH: 48px;
      --sidebarW: 280px;
      --font: -apple-system, BlinkMacSystemFont, "Segoe UI", "Noto Sans", Helvetica, Arial, sans-serif;
      --mono: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace;
      --focus: 0 0 0 2px var(--bgCanvas), 0 0 0 4px var(--accentFg);
    }
    * { box-sizing: border-box; }
    html, body { height: 100%; margin: 0; }
    body {
      font: 14px/1.5 var(--font);
      color: var(--fgDefault);
      background: var(--bgCanvas);
      -webkit-font-smoothing: antialiased;
    }
    a { color: var(--accentFg); text-decoration: none; }
    a:hover { text-decoration: underline; }
    button { font: inherit; color: inherit; }
    kbd {
      display: inline-block;
      padding: 1px 5px;
      font: 11px var(--mono);
      line-height: 1.4;
      color: var(--fgDefault);
      background: var(--bgSubtle);
      border: 1px solid var(--borderDefault);
      border-bottom-color: #bfc6cd;
      border-radius: 6px;
      box-shadow: inset 0 -1px 0 #bfc6cd;
    }
    .app { min-height: 100%; display: flex; flex-direction: column; }
    .topbar {
      height: var(--headerH);
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 0 16px;
      border-bottom: 1px solid var(--borderDefault);
      background: var(--bgCanvas);
      position: sticky;
      top: 0;
      z-index: 20;
    }
    .brand {
      display: flex;
      align-items: center;
      gap: 8px;
      color: var(--fgDefault);
      font-weight: 600;
      text-decoration: none;
      flex: 0 0 auto;
    }
    .brand:hover { color: var(--fgMuted); text-decoration: none; }
    .brand-mark {
      display: inline-grid;
      place-items: center;
      width: 28px;
      height: 28px;
      border-radius: 6px;
      background: #1f2328;
      color: #ffffff;
      font: 700 11px/1 var(--mono);
      letter-spacing: -0.02em;
    }
    .repo-chip {
      display: flex;
      align-items: center;
      gap: 6px;
      min-width: 0;
      font-size: 14px;
    }
    .repo-chip .sep { color: var(--fgMuted); }
    .repo-chip strong {
      font-weight: 600;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .badge {
      display: inline-flex;
      align-items: center;
      height: 20px;
      padding: 0 7px;
      border-radius: 2em;
      border: 1px solid var(--borderDefault);
      color: var(--fgMuted);
      font-size: 12px;
      font-weight: 500;
      background: var(--bgNeutralMuted);
    }
    .badge.green {
      color: var(--successFg);
      border-color: rgba(46, 160, 67, 0.4);
      background: var(--bgSuccessMuted);
    }
    .watch-state[hidden] { display: none; }
    .watch-state.warn {
      color: #9a6700;
      border-color: rgba(191, 135, 0, 0.4);
      background: rgba(212, 167, 44, 0.12);
    }
    .top-spacer { flex: 1; }
    .search-trigger {
      display: flex;
      align-items: center;
      gap: 8px;
      width: min(320px, 36vw);
      height: 32px;
      padding: 0 12px;
      border: 1px solid var(--borderDefault);
      border-radius: var(--radius);
      background: var(--bgInset);
      color: var(--fgMuted);
      cursor: pointer;
      text-align: left;
    }
    .search-trigger:hover { border-color: var(--fgSubtle); }
    .search-trigger:focus-visible { outline: none; box-shadow: var(--focus); }
    .search-trigger .grow { flex: 1; }
    .search-trigger .keys { display: flex; gap: 3px; }
    .btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 6px;
      height: 32px;
      padding: 0 12px;
      border-radius: var(--radius);
      border: 1px solid var(--btnBorder);
      background: var(--btnBg);
      color: var(--fgDefault);
      font-weight: 500;
      font-size: 14px;
      cursor: pointer;
      white-space: nowrap;
      text-decoration: none;
    }
    .btn:hover { background: var(--btnHoverBg); border-color: var(--fgSubtle); text-decoration: none; }
    .btn:focus-visible { outline: none; box-shadow: var(--focus); }
    .body {
      flex: 1;
      display: grid;
      grid-template-columns: var(--sidebarW) minmax(0, 1fr);
      min-height: 0;
    }
    .sidebar {
      border-right: 1px solid var(--borderDefault);
      background: var(--bgCanvas);
      overflow: auto;
      padding: 12px 0 24px;
    }
    .sidebar-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 4px 16px 10px;
      color: var(--fgMuted);
      font-size: 12px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.02em;
    }
    .tree-item {
      display: flex;
      align-items: center;
      gap: 2px;
      width: 100%;
      padding: 1px 10px 1px 0;
      border: 0;
      background: transparent;
      color: var(--fgDefault);
      font: 13px/1.4 var(--mono);
      text-align: left;
      border-left: 2px solid transparent;
      box-sizing: border-box;
    }
    .tree-item:hover { background: var(--bgNeutralMuted); }
    .tree-item.active {
      background: var(--bgAccentMuted);
      border-left-color: var(--accentEmphasis);
    }
    .tree-toggle {
      width: 18px;
      height: 18px;
      flex: 0 0 auto;
      border: 0;
      padding: 0;
      margin: 0;
      border-radius: 4px;
      background: transparent;
      color: var(--fgMuted);
      display: grid;
      place-items: center;
      cursor: pointer;
    }
    .tree-toggle:hover { background: var(--bgNeutralMuted); color: var(--fgDefault); }
    .tree-toggle svg {
      width: 12px;
      height: 12px;
      fill: currentColor;
      transition: transform .12s ease;
    }
    .tree-toggle.open svg { transform: rotate(90deg); }
    .tree-toggle.spacer {
      visibility: hidden;
      pointer-events: none;
    }
    .tree-link {
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
      flex: 1;
      padding: 4px 6px 4px 0;
      color: inherit;
      text-decoration: none;
      cursor: pointer;
    }
    .tree-link:hover { text-decoration: none; }
    .tree-item .ico {
      width: 16px;
      height: 16px;
      flex: 0 0 auto;
      color: var(--fgMuted);
      display: grid;
      place-items: center;
    }
    .tree-item.active .ico { color: var(--accentFg); }
    .tree-item .ico svg { width: 16px; height: 16px; fill: currentColor; }
    .tree-item .name {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .main { min-width: 0; overflow: auto; background: var(--bgCanvas); }
    .main-inner {
      max-width: 1280px;
      margin: 0 auto;
      padding: 16px 24px 48px;
    }
    .pathbar {
      display: flex;
      align-items: center;
      gap: 4px;
      flex-wrap: wrap;
      margin-bottom: 16px;
      font-size: 14px;
      font-family: var(--mono);
    }
    .pathbar a { color: var(--accentFg); font-weight: 600; }
    .pathbar .slash { color: var(--fgMuted); margin: 0 2px; }
    .pathbar .current { color: var(--fgDefault); font-weight: 600; }
    .file-box {
      border: 1px solid var(--borderDefault);
      border-radius: var(--radius);
      background: var(--bgCanvas);
      overflow: hidden;
    }
    .file-box-head {
      display: flex;
      align-items: center;
      gap: 12px;
      min-height: 48px;
      padding: 8px 16px;
      border-bottom: 1px solid var(--borderDefault);
      background: var(--bgSubtle);
    }
    .file-box-head .title {
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
      font-weight: 600;
    }
    .file-box-head .title .name {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .file-box-head .meta {
      color: var(--fgMuted);
      font-size: 12px;
      font-family: var(--mono);
    }
    .file-box-head .actions {
      margin-left: auto;
      display: flex;
      gap: 8px;
      flex: 0 0 auto;
    }
    .dir-table { width: 100%; border-collapse: collapse; }
    .dir-table tr { border-top: 1px solid var(--borderMuted); }
    .dir-table tr:first-child { border-top: 0; }
    .dir-table tr:hover { background: var(--bgNeutralMuted); }
    .dir-table td { padding: 8px 16px; vertical-align: middle; }
    .dir-table .name-cell {
      display: flex;
      align-items: center;
      gap: 10px;
      font-family: var(--mono);
      font-size: 14px;
    }
    .dir-table .name-cell a { color: var(--fgDefault); font-weight: 500; }
    .dir-table .name-cell a:hover { color: var(--accentFg); text-decoration: none; }
    .dir-table .type-cell {
      text-align: right;
      color: var(--fgMuted);
      font-size: 12px;
      width: 90px;
    }
    .dir-ico {
      width: 16px;
      height: 16px;
      color: var(--fgMuted);
      display: grid;
      place-items: center;
      flex: 0 0 auto;
    }
    .dir-ico svg { width: 16px; height: 16px; fill: currentColor; }
    .dir-ico.folder { color: #0969da; }
    .code-wrap { overflow: auto; background: var(--bgCanvas); }
    .code-table {
      width: 100%;
      border-collapse: collapse;
      font: 12px/20px var(--mono);
      tab-size: 2;
    }
    .code-table td { padding: 0 12px 0 0; vertical-align: top; white-space: pre; }
    .code-table .ln {
      width: 1%;
      min-width: 48px;
      padding: 0 12px;
      text-align: right;
      color: var(--fgSubtle);
      user-select: none;
      background: var(--bgCanvas);
      border-right: 1px solid var(--borderMuted);
    }
    .code-table tr:hover .ln { background: var(--bgSubtle); }
    .code-table .code { padding-left: 16px; color: var(--fgDefault); }
    .code-table .code .hljs { background: transparent; padding: 0; }
    .md {
      padding: 32px 40px;
      max-width: 980px;
      font-size: 16px;
      line-height: 1.7;
    }
    .md > :first-child { margin-top: 0; }
    .md h1, .md h2, .md h3, .md h4 {
      margin: 1.4em 0 0.6em;
      font-weight: 600;
      line-height: 1.25;
      border-bottom: 1px solid var(--borderMuted);
      padding-bottom: 0.3em;
    }
    .md h1 { font-size: 2em; }
    .md h2 { font-size: 1.5em; }
    .md h3 { font-size: 1.25em; border-bottom: 0; }
    .md p, .md ul, .md ol, .md blockquote, .md pre, .md table { margin: 0 0 1em; }
    .md a { color: var(--accentFg); }
    .md code {
      font-family: var(--mono);
      font-size: 0.875em;
      background: rgba(110, 118, 129, 0.2);
      padding: 0.2em 0.4em;
      border-radius: 6px;
    }
    .md pre {
      background: var(--bgSubtle);
      border: 1px solid var(--borderDefault);
      border-radius: var(--radius);
      padding: 16px;
      overflow: auto;
    }
    .md pre code { background: transparent; padding: 0; font-size: 13px; }
    .md blockquote {
      border-left: 0.25em solid var(--borderDefault);
      color: var(--fgMuted);
      padding: 0 1em;
    }
    .md table { border-collapse: collapse; width: 100%; }
    .md th, .md td { border: 1px solid var(--borderDefault); padding: 6px 13px; }
    .md th { background: var(--bgSubtle); font-weight: 600; }
    .md .mermaid {
      background: var(--bgSubtle);
      border: 1px solid var(--borderDefault);
      border-radius: var(--radius);
      padding: 16px;
      overflow: auto;
    }
    .md img { max-width: 100%; background: var(--bgSubtle); border-radius: var(--radius); }
    .img-wrap { padding: 20px; }
    .img-wrap img { max-width: 100%; background: var(--bgSubtle); border-radius: var(--radius); }
    .empty, .err {
      padding: 48px 24px;
      text-align: center;
      color: var(--fgMuted);
    }
    .err { color: var(--dangerFg); font-family: var(--mono); text-align: left; }
    .palette-backdrop {
      display: none;
      position: fixed;
      inset: 0;
      background: rgba(31, 35, 40, 0.45);
      z-index: 50;
      align-items: flex-start;
      justify-content: center;
      padding: 12vh 16px 16px;
    }
    .palette-backdrop.open { display: flex; }
    .palette {
      width: min(640px, 100%);
      background: var(--bgOverlay);
      border: 1px solid var(--borderDefault);
      border-radius: var(--radiusLg);
      box-shadow: var(--shadowFloat);
      overflow: hidden;
    }
    .palette-input-wrap {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 12px 14px;
      border-bottom: 1px solid var(--borderDefault);
    }
    .palette-input-wrap svg { width: 16px; height: 16px; fill: var(--fgMuted); flex: 0 0 auto; }
    .palette input {
      flex: 1;
      border: 0;
      outline: 0;
      background: transparent;
      color: var(--fgDefault);
      font: 16px var(--font);
    }
    .palette-results { max-height: min(50vh, 420px); overflow: auto; padding: 6px; }
    .palette-item {
      display: flex;
      align-items: center;
      gap: 10px;
      width: 100%;
      border: 0;
      border-radius: var(--radius);
      background: transparent;
      color: var(--fgDefault);
      padding: 10px 12px;
      cursor: pointer;
      text-align: left;
      font: 13px var(--mono);
    }
    .palette-item:hover, .palette-item.active { background: var(--bgAccentMuted); }

    .palette-section {
      padding: 8px 12px 4px;
      font-size: 11px;
      font-weight: 600;
      letter-spacing: 0.02em;
      text-transform: uppercase;
      color: var(--fgMuted);
    }
    .palette-item .meta {
      flex: 1;
      min-width: 0;
      display: flex;
      flex-direction: column;
      gap: 1px;
    }
    .palette-item .name {
      font: 13px var(--font);
      color: var(--fgDefault);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .palette-item .sub {
      font: 11px var(--mono);
      color: var(--fgMuted);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .palette-item .ext {
      flex: 0 0 auto;
      font: 10px var(--mono);
      color: var(--fgSubtle);
      background: var(--bgSubtle);
      border: 1px solid var(--borderMuted);
      border-radius: 999px;
      padding: 1px 6px;
      text-transform: lowercase;
    }
    .palette-item .label {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .palette-empty {
      padding: 20px 12px;
      text-align: center;
      color: var(--fgMuted);
      font-size: 13px;
    }
    .palette-foot {
      display: flex;
      gap: 12px;
      padding: 8px 14px;
      border-top: 1px solid var(--borderDefault);
      color: var(--fgMuted);
      font-size: 12px;
    }
    .toast {
      position: fixed;
      bottom: 24px;
      left: 50%;
      transform: translateX(-50%) translateY(20px);
      background: var(--bgOverlay);
      border: 1px solid var(--borderDefault);
      color: var(--fgDefault);
      padding: 8px 14px;
      border-radius: var(--radius);
      box-shadow: var(--shadowFloat);
      opacity: 0;
      pointer-events: none;
      transition: opacity .16s ease, transform .16s ease;
      z-index: 60;
      font-size: 13px;
    }
    .toast.show { opacity: 1; transform: translateX(-50%) translateY(0); }
    @media (max-width: 900px) {
      .body { grid-template-columns: 1fr; }
      .sidebar { display: none; }
      .search-trigger { width: 180px; }
      .md { padding: 20px 16px; }
      .main-inner { padding: 12px 12px 32px; }
    }
  </style>
</head>
<body>
  <div class="app">
    <header class="topbar">
      <a class="brand" href="/" id="brand"><span class="brand-mark">cw</span><span>code-ws</span></a>
      <div class="repo-chip">
        <span class="sep">/</span>
        <strong id="root-name">${title}</strong>
        <span class="badge green">read-only</span>
        <span class="badge watch-state" id="watch-state" hidden></span>
      </div>
      <div class="top-spacer"></div>
      <button type="button" class="search-trigger" id="btn-open" aria-label="Go to file">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M10.68 11.74a6 6 0 0 1-7.922-8.982 6 6 0 0 1 8.982 7.922l3.04 3.04a.749.749 0 0 1-.326 1.275.749.749 0 0 1-.734-.215ZM11.5 7a4.499 4.499 0 1 0-8.997 0A4.499 4.499 0 0 0 11.5 7Z"/></svg>
        <span class="grow">Go to file</span>
        <span class="keys"><kbd>⌘</kbd><kbd>K</kbd></span>
      </button>
      <button type="button" class="btn" id="btn-copy">Copy URL</button>
    </header>
    <div class="body">
      <aside class="sidebar" id="tree" aria-label="Files"></aside>
      <section class="main">
        <div class="main-inner" id="content"><div class="empty">Loading…</div></div>
      </section>
    </div>
  </div>

  <div class="palette-backdrop" id="palette" aria-hidden="true">
    <div class="palette" role="dialog" aria-modal="true" aria-label="Go to file">
      <div class="palette-input-wrap">
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M10.68 11.74a6 6 0 0 1-7.922-8.982 6 6 0 0 1 8.982 7.922l3.04 3.04a.749.749 0 0 1-.326 1.275.749.749 0 0 1-.734-.215ZM11.5 7a4.499 4.499 0 1 0-8.997 0A4.499 4.499 0 0 0 11.5 7Z"/></svg>
        <input id="palette-input" placeholder="Search files · .md .kt · recent" autocomplete="off" spellcheck="false" />
      </div>
      <div class="palette-results" id="palette-results"></div>
      <div class="palette-foot">
        <span><kbd>↑</kbd><kbd>↓</kbd> navigate</span>
        <span><kbd>↵</kbd> open</span>
        <span><kbd>esc</kbd> close</span>
        <span>.md / type:kt filter</span>
      </div>
    </div>
  </div>
  <div class="toast" id="toast">Copied</div>

  <script src="https://cdn.jsdelivr.net/npm/marked@15.0.7/marked.min.js"></script>
  <script src="https://cdn.jsdelivr.net/gh/highlightjs/cdn-release@11.11.1/build/highlight.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/mermaid@11.6.0/dist/mermaid.min.js"></script>
  <script>
${renderServeClientScript(rootJson)}
  </script>
</body>
</html>`;
}

function escapeHtml(s: string): string {
  return s
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}
