# Audit of VDO.Ninja and StreamWebRom decisions

[Español](../AUDITORIA.md) · **English**

Audited source: `github.com/steveseguin/vdo.ninja`, branch `develop`, cloned on 2026-10-17.
StreamWebRom does not copy code from VDO.Ninja: it is a from-scratch reimplementation in Go
(server) and modern dependency-free JavaScript (client), inspired by its usage model.
It is released under AGPL-3.0, the same license as the original project.

## 1. What the original repository really is

| Fact | Value |
|---|---|
| Type | Static web frontend only (HTML + JS), no build step |
| Core size | `lib.js` 76,459 lines · `webrtc.js` 26,569 · `main.js` 11,116 · `index.html` 3,552 |
| Backend | Not included: signalling lives in another repo (`websocket_server`) and by default uses VDO.Ninja public servers |
| Topology | P2P mesh: each viewer opens a connection to each sender (the sender uploads N copies) |

That is why "porting it to Go" cannot mean translating line by line: the browser only
executes JavaScript. What does make sense — and is what was done — is moving to Go everything
the original delegates to external servers, and rewriting the client so that it stays small.

## 2. Findings

| # | Severity | Finding | Evidence | How StreamWebRom solves it |
|---|---|---|---|---|
| 1 | **Critical** | The IFRAME/postMessage API runs `eval()` on received data and assigns arbitrary `innerHTML` without verifying `event.origin`. Any page that embeds the app can execute code in the victim's context (and their camera). | `main.js` ~L8732 `session.remoteInterfaceAPI`, L8750 `changeHTML` → `innerHTML`, L8975 `eval(e.data.value)` | There is no `eval` and no `innerHTML` with external data. CSP `script-src 'self'` and `frame-ancestors 'self'` block injection and clickjacking. The whole DOM is built with `textContent`. |
| 2 | High | Room passwords verified with SHA-256 truncated to 4–6 hex (16–24 bits) and a fixed default salt `"vdo.ninja"`: trivial brute force. | `webrtc.js` L431 `generateHash(..., 6)`, L1796 `session.salt = "vdo.ninja"` | Passwords with bcrypt on the server; random 120-bit host key; separate keys for viewing (`viewKey`) and for publishing over WHIP (`pushKey`), compared in constant time. |
| 3 | High | The "director" control depends on the client: whoever knows the room ID can try to act as director. | Role logic in `lib.js` | Roles are decided by the server. Every production action (`state`, `kick`, `record`, `golive`) is rejected if it does not come from the authenticated host (covered by tests). |
| 4 | Medium | 447 uses of `innerHTML` in `lib.js` (49 in `main.js`): a wide XSS surface with names and chat. | `grep -c innerHTML` | Zero `innerHTML`; the server additionally sanitises names (`<`, `>`, control chars) and limits lengths. |
| 5 | Medium | IDs with `Math.random()` (28 uses) for session/prompt identifiers. | `lib.js` L1820… | Room, peer and key IDs with `crypto/rand`. |
| 6 | Medium | Default dependency on third-party services (`wss://wss.vdo.ninja`, `piesocket.com`, `china.rtc.ninja`): privacy depends on someone else's servers. | `main.js` L582, L3890, L7625 | Everything runs in your binary: web, signalling, SFU, recording and restream. Public STUN only for IP discovery (configurable or can be disabled). |
| 7 | Medium | P2P mesh: with 4 guests and 3 outputs, each guest uploads up to 6 streams. Saturates home uplinks. | Architecture | SFU in Go (pion): each guest uploads **once** and the server forwards without decoding. |
| 8 | Low | Monolith of ~114k lines of JS with no modules and no behavioural tests. | File size | Client in ES modules (~1,500 lines), server with unit and integration tests (real WebRTC, WHIP→SFU→WHEP, recording and RTMP with ffmpeg). |
| 9 | Low | No size/rate limits on signalling messages on the self-hosted public server side. | `websocket_server` | Message size limit, per-connection rate limit, room and participant limits, read/write timeouts and disconnection of slow clients. |
| 10 | Low | Validation of external destinations/URLs. | — | Restream destinations only `rtmp://`, `rtmps://`, `srt://`; `file:`, `concat:`, `pipe:` and the `tee` muxer separators are blocked (output injection in ffmpeg). Logos/overlays only `https:` or `data:image/`. |

## 3. Features taken from StreamYard, Discord and similar tools

| Feature | Inspiration | Status |
|---|---|---|
| Backstage / stage: guests wait off camera and the host brings them up with one click | StreamYard | ✅ |
| Waiting room with admit/reject | Zoom, StreamYard | ✅ |
| 8 layouts (solo, split, grid, PiP, sidebar, featured, screen, newsroom) with shortcuts 1–8 | StreamYard | ✅ |
| Saved scenes (layout + stage + banner) in one click | OBS, StreamYard | ✅ |
| Brand: colour, theme, background, logo, full-screen overlay, on-screen names | StreamYard | ✅ |
| Banners and scrolling ticker | StreamYard | ✅ |
| Chat comments on screen | StreamYard | ✅ |
| "Starting soon" countdown | StreamYard, Restream | ✅ |
| Simultaneous multistream (YouTube, Twitch, Facebook, Kick, LinkedIn, your own RTMP/SRT) | StreamYard, Restream | ✅ (requires ffmpeg) |
| Isolated recording per guest, at original quality and without re-encoding | Riverside, StreamYard | ✅ |
| Share screen, video or local image as a source | StreamYard | ✅ |
| Speaker indicator, raise hand, emoji reactions | Discord, Meet | ✅ |
| Push-to-talk (hold Space) and M/V shortcuts | Discord | ✅ |
| Room chat persistent for the whole session | Discord | ✅ |
| Clean output for OBS (Browser Source), with optional transparent background | VDO.Ninja | ✅ |
| Standard WHIP (OBS → studio) and WHEP (studio → OBS/players) | OBS 30+, VDO.Ninja | ✅ |

## 3b. v1.1.0 audit (devices and moderation)

- Host requests to guests use an allowlist (`mute`, `unmute`, `camoff`, `camon`, `lowerhand`, `screenoff`); any other action is rejected by the server. There is no channel for executing remote code (VDO.Ninja uses `eval` on received messages).
- Privacy: the host can **turn off** a guest's microphone or camera, but **turning them on** always requires a click from the guest.
- Mute/hide "program only" and per-source volume are room state validated on the server (volume 0–2, max 200 entries); they never touch the guest's devices.
- Device names are inserted with `textContent` (no `innerHTML`).
- URL parameters only adjust local preferences; they are never sent to the server as HTML.
- New tests: `TestModeration` (allowlist, `*`/`stage` targets, permissions, volume validation). Static review of the JS modules with `scripts/jslint.py` (undeclared identifiers and non-existent imports).

## 4. Performance

- RTP forwarding without decoding; each packet is copied per subscriber without global locks.
- Immediate PLI when a viewer joins and periodic PLI (3 s) to recover the picture after loss.
- A single optional UDP port (`-udp-port`) for Docker/firewalls.
- Program composited in the host's browser at 1080p and encoded in H.264 when the browser supports it, so that ffmpeg can copy it without re-encoding.
- Single static binary (`CGO_ENABLED=0`), no runtime dependencies except ffmpeg for restream.

## 5. Known limitations

- Simulcast/SVC is not enabled yet (viewers receive the quality the guest uploads).
- The composited program recording is made by recording the `program` stream; mixing the isolated tracks in editing is left to the user (a `.mkv` per source is generated if ffmpeg is present).
- Trickle ICE is not used: offers wait up to 2–3 s to gather candidates. It is more robust, but it adds that time to connecting.
- Browser tests (real Chrome) do not run in this environment; the client was validated by syntax and the WebRTC flows were tested with real pion clients.

## v1.2.0 — remote director, scenes and effects (2026-10-17)

- **Remote control**: the server only forwards `RequestActions` actions (allowlist) with `value` ≤ 4 KB; `transfer` requires the target room to exist. The guest validates every value (resolution/FPS/bitrate range, device among those already authorised, camera controls in the `CAMCTL` allowlist). Turning on the camera, microphone or screen always requires a click from the guest; turning them off is immediate.
- **Reports** guest → host (`report`): only `devices`, `stats`, `quality`, `ack`, ≤ 16 KB, and they only reach hosts of the same room. Device IDs are shared only with the host.
- **Scenes** `outputs`: names `^[a-z0-9-]{1,16}$`, validated on the server; `/view?scene=N` still requires the viewing key.
- **Video effects**: MediaPipe Tasks Vision 0.10.14 (Apache-2.0) bundled in `web/vendor/mediapipe` (no CDN). The CSP adds only `'wasm-unsafe-eval'` (needed for WebAssembly) and `worker-src 'self'`; there is still no `unsafe-eval` and no inline scripts. Background images/videos never leave the machine (object URL; images < 1.5 MB are stored in localStorage).
- **Performance**: the effects loop uses a clock in a Worker (it is not throttled in hidden tabs), drops late ticks and limits segmentation according to its cost so as not to block signalling (found in QA: without a GPU, the tick queue delayed WebSocket messages).
- **Automated QA** (Chromium 155 headless with a fake camera): blur active, chroma background, image replacement, live statistics in the 🎛️ panel, remote switch to 360p applied (640×360), private message, scene 2 with badge on the guest and `/view?scene=2` output, transfer to another room.

## v1.3.0 — code audit and new interface (2026-10-17)

Full review of the server (Go), client (JS) and Windows app (.NET). Initial state: `go vet` clean, `go test` and `go test -race` green, JS with no syntax errors. Findings fixed (all with a test where applicable):

| # | Severity | Finding | Fix |
|---|---|---|---|
| 1 | High | `Room.Update` mutated the state before validating the whole patch: an error halfway left changes applied without being broadcast (desynchronised clients). | Deep copy + replacement only if everything validates. `TestStatePatchIsAtomic`, `TestUpdateIsAllOrNothing`. |
| 2 | High | Client reconnection: a failed attempt ended the retry loop. | `everWelcomed`: retries with backoff until `close()`. |
| 3 | High | After reconnecting, the host kept dead subscriptions and publications (frozen video, camera out of the program). | Re-subscribes and re-publishes with the same stream ID. |
| 4 | Medium | Data race: `Peer.Role` written on admission (host goroutine) and read without a lock by the guest's. | Access with `p.mu` (`role()`). |
| 5 | Medium | TOCTOU in `golive`: two simultaneous requests could start two ffmpeg sessions. | `starting` flag under the room lock. |
| 6 | Medium | WHIP/WHEP sessions were never deleted if the client did not send DELETE. | They are forgotten when the media closes. |
| 7 | Medium | When a guest left, their streams stayed in scenes 2–8, in `featured` and in `recording`. | Full cleanup in `leave`. |
| 8 | Medium | `record *` aborted at the first stream already recording or without tracks. | They are skipped and logged; clear error if there is nothing. `TestRecordAllWithoutSources`. |
| 9 | Low | A guest who left the waiting room stayed in the host's list. | `knock-cancel` message. `TestLobbyKnockCancel`. |
| 10 | Low | Leaks in the client: image `setInterval`, object URLs, late subscriptions, failed PeerConnections. | Explicit release. |
| 11 | Low | Keyboard shortcuts captured Ctrl+number and repeated keys. | Modifier/dialog filter. |
| 12 | Low | SRT key visible in the destinations list. | Only `scheme://host/••••` is shown. |
| 13 | Low | Admin token compared with `!=`. | `subtle.ConstantTimeCompare`. |
| 14 | Low | Launcher: wrong “stopped” state after restarting; embedded binary compared only by size. | Exit event tied to the current process; SHA-256. |

**Interface**: free clones of StreamYard were evaluated (e.g. Flexio, Castify, streamyard-clone-nodejs): they are React/Next prototypes with their own backend and without this project's features, so the existing interface was redesigned with StreamYard's layout, without touching the logic. QA with headless Chromium (fake camera): host + guest, layouts, backstage, Production menu, panels; no console errors. The .NET app compiles with `dotnet build` (EnableWindowsTargeting) and `dotnet publish` generates the single `.exe`.
