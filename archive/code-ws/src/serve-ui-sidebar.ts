/**
 * 侧栏拖宽: 固定 280px 会截断深层路径, 宽度必须用户可控且刷新后仍在.
 */

export const SIDEBAR_W_KEY = "code-ws.serve.sidebarW.v1";
export const SIDEBAR_W_MIN = 180;
export const SIDEBAR_W_DEFAULT = 280;
export const SIDEBAR_W_MAX_RATIO = 0.6;

/** 夹在最小宽度与视口比例上限之间, 避免主区被拖没. */
export function clampSidebarW(px: number, viewW: number): number {
  const max = Math.max(SIDEBAR_W_MIN, Math.floor(viewW * SIDEBAR_W_MAX_RATIO));
  return Math.min(max, Math.max(SIDEBAR_W_MIN, Math.round(px)));
}

export const CLIENT_SIDEBAR_RESIZE = `
    const SIDEBAR_W_KEY = ${JSON.stringify(SIDEBAR_W_KEY)};
    const SIDEBAR_W_MIN = ${SIDEBAR_W_MIN};
    const SIDEBAR_W_DEFAULT = ${SIDEBAR_W_DEFAULT};
    const SIDEBAR_W_MAX_RATIO = ${SIDEBAR_W_MAX_RATIO};

    function clampSidebarW(px, viewW) {
      const max = Math.max(SIDEBAR_W_MIN, Math.floor(viewW * SIDEBAR_W_MAX_RATIO));
      return Math.min(max, Math.max(SIDEBAR_W_MIN, Math.round(px)));
    }

    function applySidebarW(px) {
      const w = clampSidebarW(px, window.innerWidth);
      document.documentElement.style.setProperty("--sidebarW", w + "px");
      const handle = $("sidebar-resizer");
      if (handle) {
        handle.setAttribute("aria-valuenow", String(w));
        handle.setAttribute("aria-valuemin", String(SIDEBAR_W_MIN));
        handle.setAttribute("aria-valuemax", String(Math.max(
          SIDEBAR_W_MIN,
          Math.floor(window.innerWidth * SIDEBAR_W_MAX_RATIO),
        )));
      }
      return w;
    }

    function readSidebarW() {
      const raw = getComputedStyle(document.documentElement).getPropertyValue("--sidebarW");
      const n = parseFloat(raw);
      return Number.isFinite(n) ? n : SIDEBAR_W_DEFAULT;
    }

    function loadSidebarW() {
      try {
        const raw = localStorage.getItem(SIDEBAR_W_KEY);
        if (raw == null) {
          applySidebarW(SIDEBAR_W_DEFAULT);
          return;
        }
        const n = Number(raw);
        applySidebarW(Number.isFinite(n) ? n : SIDEBAR_W_DEFAULT);
      } catch {
        applySidebarW(SIDEBAR_W_DEFAULT);
      }
    }

    function saveSidebarW(px) {
      try { localStorage.setItem(SIDEBAR_W_KEY, String(px)); } catch {}
    }

    function bindSidebarResize() {
      const handle = $("sidebar-resizer");
      if (!handle) return;
      let startX = 0;
      let startW = 0;

      function onMove(e) {
        applySidebarW(startW + (e.clientX - startX));
      }

      function onUp(e) {
        document.body.classList.remove("sidebar-resizing");
        handle.releasePointerCapture(e.pointerId);
        handle.removeEventListener("pointermove", onMove);
        handle.removeEventListener("pointerup", onUp);
        handle.removeEventListener("pointercancel", onUp);
        saveSidebarW(applySidebarW(startW + (e.clientX - startX)));
      }

      handle.addEventListener("pointerdown", (e) => {
        if (e.button !== 0) return;
        e.preventDefault();
        startX = e.clientX;
        startW = readSidebarW();
        document.body.classList.add("sidebar-resizing");
        handle.setPointerCapture(e.pointerId);
        handle.addEventListener("pointermove", onMove);
        handle.addEventListener("pointerup", onUp);
        handle.addEventListener("pointercancel", onUp);
      });

      handle.addEventListener("dblclick", () => {
        saveSidebarW(applySidebarW(SIDEBAR_W_DEFAULT));
      });

      handle.addEventListener("keydown", (e) => {
        const step = e.shiftKey ? 48 : 16;
        if (e.key === "ArrowLeft") {
          e.preventDefault();
          saveSidebarW(applySidebarW(readSidebarW() - step));
          return;
        }
        if (e.key === "ArrowRight") {
          e.preventDefault();
          saveSidebarW(applySidebarW(readSidebarW() + step));
        }
      });

      window.addEventListener("resize", () => {
        if (document.body.classList.contains("sidebar-resizing")) return;
        applySidebarW(readSidebarW());
      });
    }
`;
