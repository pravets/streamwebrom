// Shared sender limits. These do not change the 16:9 compositor/OBS raster.

// Shipped defaults of the device panel. A source that keeps them expressed no
// preference, so the room bandwidth mode stays in charge of it.
export const PANEL_DEFAULTS = { quality: "720", fps: "30" };

// URL knobs that express the same intent, shared by the guest page and the
// studio camera so the "did somebody ask for this?" rule cannot drift.
export const TUNING_PARAMS = ["bitrate", "vb", "quality", "height", "h", "fps", "framerate", "fr"];

/** The hasParam probe for explicitQuality, built from a URLSearchParams. */
export const explicitFromParams = (params) => () => TUNING_PARAMS.some((n) => params.has(n));

/**
 * Did somebody explicitly ask for this quality? URL parameters, the director
 * panel or the sender's own panel all count; the shipped defaults do not,
 * otherwise the room mode could never protect a source. `hasParam` is injected
 * so the rule stays testable without a browser.
 */
export function explicitQuality(prefs = {}, hasParam = () => false) {
  if (hasParam()) return true;
  // The user touched the quality/fps selects: even a choice equal to the
  // shipped default is intent, not "never asked".
  if (prefs.tuned) return true;
  const quality = String(prefs.quality ?? PANEL_DEFAULTS.quality);
  const fps = String(prefs.fps ?? PANEL_DEFAULTS.fps);
  return quality !== PANEL_DEFAULTS.quality || fps !== PANEL_DEFAULTS.fps;
}

/**
 * Remembers explicit quality intent across reconnects. A director command or a
 * panel change made mid-call lives only in the publication options, and a
 * reconnect drops the publication before the flag can be read from it.
 */
export function explicitMemory() {
  let on = false;
  return {
    mark() { on = true; },
    get on() { return on; },
    reset() { on = false; },
  };
}

export function senderProfile(mode, opts = {}, settings = {}) {
  const raw = { videoKbps: opts.videoKbps || 4000, audioKbps: opts.audioKbps || 128, fps: opts.fps || 30, scale: 1 };
  // Explicit per-source tuning (director panel, guest panel, URL params) and
  // local content (program, media files) override the room bandwidth caps.
  if (mode === "quality" || opts.kind === "program" || opts.kind === "media" || opts.explicit) return raw;
  const eco = mode === "eco", screen = opts.kind === "screen";
  const height = screen ? 1080 : eco ? 540 : 720;
  const w = Number(settings.width) || 0, h = Number(settings.height) || 0;
  return {
    videoKbps: Math.min(raw.videoKbps, screen ? eco ? 1500 : 2500 : eco ? 900 : 1500),
    audioKbps: Math.min(raw.audioKbps, opts.stereo ? eco ? 96 : 128 : eco ? 48 : 64),
    fps: Math.min(raw.fps, screen ? eco ? 8 : 15 : eco ? 20 : 24),
    scale: Math.max(1, Math.max(w, h) / (height * 16 / 9), Math.min(w, h) / height),
  };
}

export function needsOutputSource(info, state, only, sceneName = "1") {
  if (only) return info.id === only;
  if (info.kind === "program") return false;
  const output = ["", "0", "1"].includes(sceneName) ? state : state.outputs?.[sceneName];
  return (output?.stage || []).includes(info.id);
}
