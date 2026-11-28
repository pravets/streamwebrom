// Forced landscape for phone calls (guest page). Android Chrome can lock the
// screen orientation only while the page is fullscreen, so immersive.js calls
// the lock right after requestFullscreen succeeds and releases it on exit.
// iOS Safari and other browsers without the lock API get a gentle "rotate your
// phone" hint instead: a CSS rotation would break fixed positioning and would
// never rotate the outgoing video.
// No imports: the hint text lives in the markup (data-i18n) so the language
// core keeps it translated, and this module stays trivially testable.

export function orientationSupported() {
  const o = typeof screen !== "undefined" ? screen.orientation : null;
  return !!o && typeof o.lock === "function";
}

/** Phone/tablet browser: forced landscape makes sense there, not on desktop. */
export function isMobileUA(ua = "", touchPoints = 0) {
  return /Mobi|Android|iPhone|iPad/i.test(ua) || touchPoints > 1;
}

/**
 * Resolve the initial landscape mode: the URL wins, then the saved choice;
 * with neither, a mobile browser defaults to on (the user can still untick
 * the prejoin checkbox). `stored` is the raw localStorage value or null.
 */
export function defaultLandscape({ urlOn = false, urlOff = false, stored = null, mobile = false } = {}) {
  if (urlOn) return true;
  if (urlOff) return false;
  if (stored !== null) return stored === "1";
  return mobile;
}

/** Lock to landscape. Resolves true on success; NotSupportedError,
 * SecurityError and friends resolve false instead of throwing. */
export async function lockLandscape() {
  if (!orientationSupported()) return false;
  try { await screen.orientation.lock("landscape"); return true; }
  catch { return false; }
}

/** Release the lock; safe when nothing was locked or the API is absent. */
export function unlockLandscape() {
  try { screen.orientation?.unlock?.(); } catch {}
}

/**
 * Show `node` while landscape mode is enabled, the device is portrait and the
 * browser cannot lock the orientation itself; hide it in landscape. Returns a
 * controller with setEnabled(on) and destroy().
 */
export function installOrientationHint(node) {
  let enabled = false;
  const mq = typeof window !== "undefined" && window.matchMedia ? window.matchMedia("(orientation: portrait)") : null;
  const update = () => {
    const show = enabled && !!mq?.matches && !orientationSupported();
    node.classList.toggle("hidden", !show);
  };
  if (mq) { if (mq.addEventListener) mq.addEventListener("change", update); else mq.addListener?.(update); }
  update();
  return {
    setEnabled(on) { enabled = !!on; update(); },
    destroy() {
      if (mq) { if (mq.removeEventListener) mq.removeEventListener("change", update); else mq.removeListener?.(update); }
      node.classList.add("hidden");
    },
  };
}