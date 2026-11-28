// The in-page cinema mode works even when the device has no Fullscreen API.
export function immersive(root, button, exit, opts = {}) {
  let timer, active = false, pending = 0;
  const wake = () => {
    root.classList.remove("controls-idle"); clearTimeout(timer);
    if (active) timer = setTimeout(() => {
      if (!root.contains(document.activeElement) || document.activeElement === root || document.activeElement?.tagName === "VIDEO") root.classList.add("controls-idle");
    }, 3000);
  };
  function apply(on) {
    active = on; root.classList.toggle("cinema", on);
    button.setAttribute("aria-pressed", String(on));
    exit.classList.toggle("hidden", !on); wake();
    window.dispatchEvent(new Event("resize"));
  }
  const leave = () => {
    apply(false);
    opts.onExitFullscreen?.();
    if (document.fullscreenElement === root) document.exitFullscreen?.().catch(() => {});
  };
  const enter = async (native = opts.native !== false) => {
    apply(true); button.blur();
    if (native) {
      pending++;
      try {
        if (root.requestFullscreen) {
          await root.requestFullscreen();
          // leave() may have cancelled cinema mode while the request was
          // pending: do not lock orientation for a cancelled entry, and undo
          // the fullscreen the late request just granted.
          if (!active) { document.exitFullscreen?.().catch(() => {}); return; }
          // The orientation lock (guest landscape mode) is only legal inside
          // fullscreen, so it runs after the request actually succeeded.
          opts.onFullscreen?.();
        }
      } catch { /* cinema still fills the browser */ }
      finally { pending--; }
    }
  };
  const toggle = () => { if (active) leave(); else enter(); };
  // A fullscreenchange that lands while an entry is still in flight belongs to
  // the exit of an earlier entry (or to the transition itself). Tearing the
  // newer session down for it would cancel the entry the user just asked for.
  const changed = () => { if (pending > 0) return; if (!document.fullscreenElement) leave(); };
  const key = e => { if (e.key === "Escape" && active) leave(); else wake(); };
  button.addEventListener("click", toggle); exit.addEventListener("click", leave);
  document.addEventListener("fullscreenchange", changed); document.addEventListener("keydown", key);
  root.addEventListener("pointermove", wake); root.addEventListener("pointerdown", wake);
  return { enter, leave, destroy() {
    leave(); clearTimeout(timer); button.removeEventListener("click", toggle); exit.removeEventListener("click", leave);
    document.removeEventListener("fullscreenchange", changed); document.removeEventListener("keydown", key);
    root.removeEventListener("pointermove", wake); root.removeEventListener("pointerdown", wake);
  } };
}
