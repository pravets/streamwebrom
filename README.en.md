# StreamWebRom

[Español](README.md) · **English** · [Русский](README.ru.md)

**Version 1.12.0:** Advanced capture of windows, tabs and the system with isolated audio on Windows x64, keeping the studio interface. Web source and sound selector, with protection against voice feedback. [Advanced capture](docs/en/ADVANCED-CAPTURE.md), [audio and background](docs/en/AUDIO-AND-BACKGROUND.md), [apps](mobile/README.md) and [changes](CHANGELOG.md).

Code and downloads: [GitHub](https://github.com/webr0m/streamwebrom) · [GitLab](https://gitlab.com/ridrogo/streamwebrom) · [Gitea](https://gitea.com/webrom/streamwebrom).

**Live streaming studio in a single binary.** Guests by link, clean output for OBS,
multistream to YouTube/Twitch/Facebook/Kick, isolated recording per guest and live branding.
Server in Go (WebRTC SFU with [pion](https://github.com/pion/webrtc)), web client with no dependencies.

Inspired by VDO.Ninja, StreamYard and Discord — rewritten from scratch, audited and tested.
Since v1.3 the studio has a **StreamYard-style interface**: scenes on the left, canvas and layouts in the center, backstage and button bar at the bottom, and panels with an icon rail on the right.
See [docs/en/AUDIT.md](docs/en/AUDIT.md).

## What you get

| | |
|---|---|
| 🎥 **Guests with a single link** | No accounts and nothing to install. Camera/microphone test, quality and noise suppression before entering. |
| 🚪 **Backstage and waiting room** | Guests wait off camera; you bring them to the stage with one click. Admit or reject each one. |
| 🎬 **8 layouts + scenes** | Solo, split, grid, PiP, side, featured, screen and news. Shortcuts 1–8. Save scenes. |
| 🏷️ **Live branding** | Color, theme, background, logo, PNG overlay, names, banners, ticker, on-screen comments and countdown. |
| 📡 **Multistream** | Several RTMP/RTMPS/SRT destinations at once with a single ffmpeg. H.264 is copied without re-encoding. |
| 🧩 **Real OBS** | Browser Source of the program (optionally transparent) or of each guest. **WHIP** to send OBS to the studio and **WHEP** to receive in OBS/players. |
| ⏺️ **Isolated recording** | Each source is recorded separately, at original quality, on the server side (`.ivf/.ogg/.h264` → `.mkv`). |
| 💬 **Community** | Chat, reactions, raise hand, speaking indicator, push-to-talk (Space), M/V shortcuts. |
| ⚡ **Fast** | SFU: each guest uploads only once. RTP forwarding without decoding, immediate PLI, minimal jitter buffer. |
| 🎧 **Devices like VDO.Ninja** | Pick camera, microphone and headphones inside the app (also mid-call, without dropping). Resolution up to 4K, FPS, gain, echo/noise/AGC, HQ stereo, codec, zoom/focus/exposure/torch/background blur depending on what the camera supports. |
| 🛡️ **Host control** | Mute stage, mute everyone, mute someone on the program or hide their image also in backstage, volume per source, turn cameras off, request audio/camera, lower hands, kick. |
| 🎛️ **Director like VDO.Ninja** | Per guest: resolution, FPS and bitrate of what they send, change their camera or microphone, zoom/focus/exposure, gain and echo/noise, live stats, private message, prevent them from hearing, reload and move to another room. |
| 🎞️ **Scenes 1–8** | Program + 7 extra scenes, each with its own people and layout, with its own OBS link (`scene=N`) and preview. |
| ✨ **Video effects** | Background blur, virtual background (image, video, gradients, green/blue chroma) or replacing the camera with an image/video. Processed on the device, no CDN. |
| 📺 **Virtual screen** | Clean program window for projector, second monitor, window capture or OBS Virtual Camera (double click = fullscreen). |
| 🔐 **Secure** | Server-validated roles, bcrypt, separate keys (host / view / publish), strict CSP, rate-limit. |

## Quick start

Download the binary for your platform from **Releases** and run it:

```bash
# Just this computer (http://localhost:8443)
./streamwebrom -open

# Phones and other PCs on your network (self-signed HTTPS, accept the warning once)
./streamwebrom -self-signed -udp-port 50000

# Public server with Let's Encrypt certificate
sudo ./streamwebrom -autocert live.midominio.com -addr :443 -udp-port 50000 -public-ip 203.0.113.10
```

**Windows:** download `StreamWebRom-Windows-x64.exe` (native .NET 8 app). It stays in the tray,
shows the links and opens the studio. For multistream install ffmpeg (`winget install ffmpeg`)
or leave `ffmpeg.exe` next to the executable.

**Docker:** `docker compose up -d`

**Go:** `go install github.com/webr0m/streamwebrom/cmd/streamwebrom@latest`

**Installable web app (PWA):** open the studio in Chrome/Edge and use *Install app*; it works on PC and mobile just like VDO.Ninja.

**Android:** the delivery includes a test APK and an unsigned one for distribution. **iPhone/iPad:** SwiftUI/WKWebView project and simulator build in CI; an IPA for devices requires Apple signing. Both clients connect to your HTTPS server. Instructions and limits in [mobile/README.md](mobile/README.md).

**Before going live:** open `/diagnostics` to check browser, camera/microphone, server connection, ffmpeg and TURN configuration.

### Publishing it on your domain from your PC (Cloudflare Tunnel)

Ideal when your hosting is shared (cPanel) and cannot run video servers.

1. In Cloudflare Zero Trust create a tunnel with a public hostname (e.g. `stream.webrom.cc`) → `http://localhost:8443`.
2. In the Windows app check *Publish to the internet*, type the domain and paste the tunnel token (it is stored encrypted with DPAPI). The app downloads `cloudflared` and runs it.
3. Forward port **UDP 50000** on your router to your PC for direct connections. For restrictive mobile networks, CGNAT or blocked UDP, also configure a **TURN relay with TCP/TLS**, preferably on port 443. The web and the signaling go through Cloudflare Tunnel; the HTTP tunnel does not carry WebRTC audio/video. Instructions: [mobile networks](docs/en/MOBILE-NETWORKS.md).

On Linux/macOS: `streamwebrom -addr 127.0.0.1:8443 -public-url https://stream.tudominio.com -udp-port 50000` and `cloudflared tunnel run --token …`.

## Using it with OBS

1. Open the studio → **Create studio** → invite people with **🔗 Invite**.
2. **🧩 OBS** tab → copy **Full program**.
3. OBS → Sources → **+** → **Browser** → paste the URL · 1920×1080 · check *Control audio via OBS*.

| URL | What for |
|---|---|
| `/view?room=R&scene=1#key=VIEWKEY` | Full program with layout, branding and banners |
| `/view?room=R&scene=1&transparent=1#key=…` | The same with a transparent background, for your OBS scenes |
| `/view?room=R&stream=ID#key=…` | A single guest fullscreen (`&cover=1` crops, `&mute=1` without audio) |
| `POST /whip/R/obs` + Bearer `pushKey` | OBS → Settings → Stream → Service **WHIP** |
| `POST /whep/R/program` + Bearer `viewKey` | WHEP players |

The keys travel in the `#` fragment, which the browser never sends to the server or to the logs.

WHIP to the reserved ID `program` requires `hostKey`; use `pushKey` for sources such as `obs` or `cam`. DELETE/PATCH resources require the corresponding Bearer. Send complete SDP: trickle ICE PATCH returns 501 because it is not implemented.

## URL options (VDO.Ninja style)

**Guest** `/guest?room=R&…`

| Parameter | Effect |
|---|---|
| `label=Ana` · `autostart` | Pre-filled name · enter without the preview screen |
| `password=` | Room password |
| `videodevice=` · `audiodevice=` · `outputdevice=` | Device by number (1, 2…), part of the name (`obs`, `logitech`) or `0` for none |
| `quality=0/1/2` · `height=1080` · `fps=60` | Resolution and frames |
| `bitrate=6000` · `codec=h264/vp8/vp9/av1` | Send quality |
| `stereo` · `denoise=0` · `echo=0` · `autogain=0` · `gain=150` | Professional audio |
| `novideo` · `noaudio` · `muted` · `videomute` | Audio only, video only, enter muted |
| `screenshare` | Enter with screen sharing |
| `effects=blur/bg/replace` · `blur` · `bgpreset=green` | Video effect (blur, virtual background, replacement) |

**Output for OBS** `/view?room=R&…#key=VIEWKEY`

| Parameter | Effect |
|---|---|
| `scene=1` | Full program · `transparent=1` transparent background |
| `scene=2` … `scene=8` | Extra scenes built in the studio |
| `stream=ID` | A single camera (each source has its own **🔗 OBS** button) |
| `cover=1` · `mirror=1` · `rotate=90` · `bg=000000` | Framing |
| `mute=1` · `volume=50` · `audioonly=1` | Audio |
| `label=1` · `stats=1` · `status=0` | Name, statistics, hide status |

## Configuration

Every option is a flag or an environment variable `SWR_<NAME>` (e.g. `-udp-port` = `SWR_UDP_PORT`).

| Flag | Default | Description |
|---|---|---|
| `-addr` | `:8443` | Listen address |
| `-self-signed` | `false` | HTTPS with a persistent self-signed certificate (LAN) |
| `-autocert` | | Domain for Let's Encrypt |
| `-tls-cert` / `-tls-key` | | Your own certificate |
| `-udp-port` | `0` | A single UDP port for all the media (recommended with firewall/Docker) |
| `-public-ip` | | Announced public IP (1:1 NAT, VPS) |
| `-stun` / `-turn` / `-turn-user` / `-turn-pass` | Google and Cloudflare STUN | ICE servers |
| `-ffmpeg` | `ffmpeg` | Binary for multistream and recording mux |
| `-data` | `streamwebrom-data` | Recordings and certificates |
| `-origins` | same host | Allowed WebSocket origins (`*` = any) |
| `-max-rooms` / `-max-peers` | `500` / `64` | Limits |
| `-msg-rate` / `-msg-burst` / `-max-msg` | `30` / `60` / `256KiB` | Signaling anti-abuse |
| `-admin-token` | | Enables `/api/admin/stats` and `/metrics` (Prometheus) |
| `-open` | `false` | Opens the browser on start |

## Development

```bash
make run          # local server
make test         # vet + tests + JS syntax
make test-race    # race detector (requires cgo)
make all          # binaries for linux/mac/windows (amd64 and arm64)
make launcher     # .NET Windows app with the embedded server
```

On PowerShell: `./scripts/build.ps1` generates the six servers, the Windows launcher and `dist/SHA256SUMS.txt` (requires Go and the .NET 8 SDK). `./scripts/build-android.ps1` generates the APKs with JDK 17 and Android SDK 35. After adding artifacts, recompute the checksums before publishing.

```
cmd/streamwebrom     entry point
internal/sfu         SFU: publishing, subscription, RTP forwarding, PLI
internal/studio      rooms, roles, production state, signaling protocol
internal/server      HTTP, WebSocket, API, WHIP/WHEP, security headers
internal/restream    RTMP/SRT multistream with ffmpeg
internal/record      isolated recording per source
internal/tlsutil     self-signed certificates for LAN
web/                 client (ES modules, no build): studio, guest, OBS output
windows/             native .NET 8 launcher for Windows
```

The tests include real integration: one WebRTC client publishes via WHIP, another receives via WHEP,
the stream is recorded and a VP8+Opus → H.264+AAC multistream is made to a local RTMP server.

## License

AGPL-3.0-or-later. If you offer StreamWebRom as a service to other people, you must offer them its source code.
