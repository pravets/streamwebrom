# StreamWebRom 1.5.0 Audit

[Español](../AUDITORIA-1.5.md) · **English**

Date: October 18, 2026. Base reviewed: Gitea/GitLab `f5f7127` (1.4.0).
This audit corresponds to the StreamWebRom code; `AUDITORIA.md` keeps the historical VDO.Ninja analysis.
The review and the tests do not demonstrate the absence of every possible defect.

## Fixes applied

| Priority | Defect or risk | Change and evidence |
|---|---|---|
| High | When a guest left, `dropStreams` called `Room.Stream` inside `Room.Update`, attempting to take the same lock twice. | Map lookup under the existing lock. Test with a completion deadline and reconnection of a source. |
| High | Closing a replaced publication emitted `stream-removed` for the new publication with the same ID. | Source identity is checked before emitting the event. |
| High | Two simultaneous negotiations could pass the owner check and take over the same ID. | ID reservation during negotiation, owner and capacity checks; logical concurrency tests. |
| High | Guests could publish a camera using the reserved `program` ID. | The ID requires host; WHIP towards `program` requires `hostKey`. |
| High | WHIP/WHEP DELETE/PATCH did not authenticate and did not bind the resource to room, stream and protocol. | Mandatory Bearer and full-path match. Cross-key and fake-path tests. |
| High | An OBS output kept dead subscriptions when reconnecting and could end up frozen. | Connections and audio are released, and sources are rediscovered and resubscribed. Chromium test with WebSocket disconnection and check of newly decoded frames. |
| Medium | Old restream responses could turn off the state of a new stream; stopping while starting did not cancel the pending start. | Session generation and identity check; state update under lock. |
| Medium | UDP sockets could occupy ports that ffmpeg needed for RTP or RTCP, especially on Windows. | Reservation of RTP/RTCP port pairs during the assignment of all tracks and senders. Reservation test and real RTMP integration. |
| Medium | `ffmpeg` could return full URLs with RTMP keys in the diagnostics broadcast to the studio. | The state uses messages without secrets and only accepts known progress fields. Test with a fictitious key. |
| Medium | A client could accumulate publications without limit. | Maximum of eight publications per peer and capacity per room, including pending WHIP negotiations. |
| Medium | A cancelled request could end up creating media and returning a successful response. | ICE cancellation is propagated and resources are closed. Cancelled-context test. |
| Medium | Closing the hub left participant sockets open. | Room closure closes participants, waiting guests, media and restream. Closed rooms reject new publications. |
| Medium | HTTP bodies were silently truncated; concatenated JSON and SDP types with fake prefixes were accepted. | Effective limits, a single JSON entity and exact SDP MIME type. HTTP rejection tests without creating rooms. |
| Medium | Invalid environment variables were ignored; negative UDP ranges or ranges above 65535 ended up converted to uint16. | Explicit errors and range validation, mutually exclusive modes, TTL, finite rates, IP and public URL. |
| Medium | Passwords of 73–128 bytes passed validation but bcrypt rejected them afterwards. | Real limit of 72 bytes on the server and in the form, including multibyte characters. |
| Medium | Corrupt JSON or full storage could make the landing page or the studio unusable; the key was hidden even when it had not been saved. | Reads with fallback, controlled writes and preservation of the fragment when the key could not be saved. Node and browser tests. |
| Medium | `Signal.close()` did not cancel an already scheduled retry; SDP requests could remain queued and be replayed after reconnecting. | Timer cancellation, immediate offline rejection and discarding of requests from previous sessions. Seven Node tests cover signalling and storage. |
| Medium | Expulsion/rejection could trigger automatic reconnection with the previous pass. | The local pass is invalidated and retries are stopped; a password error allows a new manual attempt. |
| Medium | The PWA saved arbitrary URLs with parameters and deleted caches belonging to others when activating a version. | Allowlist of resources, keys without query, cleanup limited to `swr-` caches and an explicit offline response. |
| Low | ICE PATCH responded with success even though it ignored the candidates. | Explicit 501 response; send complete SDP offers. |
| Low | The colour expression accepted five- and seven-digit CSS hex values. | Only 3, 4, 6 or 8 digits are accepted. Rejection tests. |
| Low | Saved scenes accepted invalid layouts, repeated IDs and tag lists with no entry limit. | Validation of scenes, IDs, layouts, names, texts and tags; atomic rejection tested. |
| Low | Restarting a recording within the same second could overwrite the previous files. | Names with nanoseconds; mux with a two-minute deadline. |
| Low | ICE listeners, statistics timers or audio measurement nodes were left retained. | Cleanup on ICE completion, session change or track end. |
| Low | The new launcher shipped with the tunnel enabled and a third-party domain preconfigured; an undecryptable token could disable HTTPS. | Tunnel disabled by default, empty domain and verification of the decrypted token. The existing configuration is preserved. |
| Low | A launcher could be published without the embedded server. | The release build fails if the Go binary is missing. |

## Measured optimization

`Room.Snapshot` and `Room.Update` copied the state by serializing and deserializing JSON. Now slices, maps and pointers are copied explicitly, while preserving the isolation of atomic updates.

Local benchmark, Windows amd64, Ryzen 7 5700X3D, Go 1.27.2, state with 25 sources and 25 banners:

| Method | Time per copy | Memory | Allocations |
|---|---:|---:|---:|
| Direct copy | 466 ns | 1,536 B | 4 |
| Previous JSON | 48,259 ns | 11,526 B | 22 |

That is approximately 104 times less time in **this operation** and 87% less allocated memory. It is not a measure of the overall streaming performance. Repeat with `go test -run '^$' -bench StateCopy -benchmem ./internal/studio`.

Host keys, random 120-bit and without server-side persistence, are verified with SHA-256 and constant-time comparison. Human guest passwords still use bcrypt. Password hashing when creating rooms is performed outside the hub lock.

## Incorporated suggestions

1. **Upfront diagnostics:** `/diagnostics` checks secure context, WebRTC, device API, server, ffmpeg availability and TURN configuration. The optional camera/microphone test releases the tracks when it finishes. It does not simulate an external ICE connection.
2. **Mobile clients:** Android and iPhone/iPad projects with a configurable server URL, camera/microphone permissions, mandatory HTTPS and restriction to the selected origin. The Go server remains on the PC/VPS.
3. **Repeatable builds:** PowerShell scripts for six server binaries, Windows launcher, test APK, unsigned distribution APK and checksums.
4. **Continuous testing:** Go/Node, Chromium and WebKit regressions; Android and iOS builds on GitHub; Go tests and binaries on GitLab.

## Validation and limits

- `go vet ./...` and `go test -count=1 ./...`, including WHIP → SFU → WHEP/recording integration and VP8+Opus → ffmpeg → RTMP.
- Syntax of all JS modules and `sw.js`, `scripts/jslint.py` and seven Node tests.
- Real Chromium with synthetic media: corrupt storage, host camera, admission from the lobby, guest publication, stage, OBS output, reconnection with newly decoded frames and diagnostics. No page errors. WebKit verifies the diagnostics page.
- APK built, test signature verified, installation and launch on an Android 15 emulator without an `AndroidRuntime` exception.
- Windows launcher built with .NET 8; servers for Windows/Linux/macOS, amd64 and arm64.
- `govulncheck`: zero reachable vulnerabilities. Module advisory GO-2026-5932 corresponds to `x/crypto/openpgp`, a package this application does not import; it has no fixed version because it is unmaintained. The analysis does not cover every native dependency or the operating system.
- The race detector passed on Linux in [GitHub CI](https://github.com/webr0m/streamwebrom/actions/runs/39109201475), together with tests on Windows and macOS and the Chromium/WebKit test. The local Windows machine had no C compiler.
- The iOS project built correctly with Xcode in [the macOS workflow](https://github.com/webr0m/streamwebrom/actions/runs/39109201483/job/117621469767), producing the simulator app. A signed app for devices/TestFlight requires an Apple team, certificates and the owner's profile; an installable IPA is not generated from Windows.
- The delivered APKs are expressly distinguished: `debug` is for testing; `unsigned` requires your distribution signature. They do not include a media server inside the phone.
- A call on physical iPhone/Android devices was not tested, nor every combination of OBS, codecs, NAT and TURN. Mobile screen sharing and background execution are not implemented in the native clients.

## Recommended future work

| Priority | Improvement | Reason |
|---|---|---|
| High | Disk quotas, retention and authenticated download of recordings | Avoid filling the server and allow managing files from the studio. |
| High | Temporary TURN credentials and per-IP authentication limits | Reduce relay and password abuse in public deployments. |
| High | Testing on physical phones and real mobile networks | Verify audio routes, headphones, rotation, permissions and reconnection across network changes. |
| Medium | Simulcast/SVC and per-subscriber layer selection | Adapt consumption to viewers with different connections. |
| Medium | Loss, RTT and bandwidth metrics per room | Diagnose call quality during a stream. |
| Medium | Recoverable local recording and chunked upload | Improve recovery from network drops; it requires a storage and synchronisation design. |

These extensions are identified, not presented as implemented features. The native permission APIs follow the [Android](https://developer.android.com/reference/android/webkit/PermissionRequest) and [WebKit](https://developer.apple.com/documentation/webkit/wkuidelegate/webview(_:requestmediacapturepermissionfor:initiatedbyframe:type:decisionhandler:)) references.
