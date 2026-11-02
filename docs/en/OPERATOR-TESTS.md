# Verification with mobile operators in Chile

[Español](../PRUEBAS-OPERADORES.md) · **English** · [Русский](../ru/OPERATOR-TESTS.md)

Status as of 18 October 2026. A test from another connection, a simulator or a CI server does not certify a real mobile network. The result corresponds to the combination of operator, line, plan, zone, phone, browser and technology tested; it does not guarantee every condition of a carrier.

## Available evidence

| Operator | Video with Cloudflare TLS/443 | Video with local relay TLS/443 | Backstage entry | Evidence |
|---|---|---|---|---|
| Movistar Chile | Confirmed | Confirmed | Confirmed | User, Chrome on a phone with mobile data, 18-10-2026, version 1.6.1; automatic entry confirmed by the user in 1.6.3. Exact technology of the latest test not recorded; 4G and 5G still need to be separated. |
| Entel | Pending | Pending | Pending | A test from a real line is required. |
| Claro / VTR | Pending | Pending | Pending | Record the brand and line tested separately. |
| WOM | Pending | Pending | Pending | A test from a real line is required. |
| Other operators / MVNOs | Pending | Pending | Pending | Add each brand used; sharing the host network does not replace its test. |

The initial scope takes as a reference the mobile data operators identified by [Subtel in 2026](https://www.subtel.gob.cl/chile-supera-las-10-millones-de-conexiones-5g-y-cuenta-con-el-internet-fijo-mas-barato-de-america-latina/). This table records project tests, not a Subtel or operator certification.

## Reproducible test per line

1. Update StreamWebRom and reload Chrome. Disable Wi-Fi and check that the phone is using mobile data. Note whether it shows 4G/LTE or 5G; repeat with each available technology, without considering one proven on the basis of the other.
2. Open `https://stream.webrom.cc/diagnostics?network=tls`. Press "Test video to the server" and record the result. The camera and microphone are not used.
3. Repeat at `https://stream.webrom.cc/diagnostics?network=local`. Both relays must be configured by the host.
4. From 1.6.3, enter with the normal link and the default automatic options, without selecting a route. The system must check the media and choose a relay before entering. The host must receive the camera and microphone, and the guest must see and hear the host. Hold a conversation for at least two minutes.
5. Check admission to the backstage and joining the stage. Interrupt the data briefly and verify that it reconnects without getting stuck again. From 1.6.4, if "Reconnect" appears, press it and check that it returns without reloading or choosing a relay; keep the camera and mute state.
6. Repeat both routes. If one fails, record the exact message and keep the other as a fallback. Do not change APN or IPv6 during the baseline test.

## Recording a new test

```text
Local date and time:
Version shown by the server:
Operator / commercial brand:
4G/LTE or 5G:
City or region (optional; no exact address):
Phone model and Chrome version:
Wi-Fi off: yes/no
Cloudflare TLS/443: pass/fail + message
Local relay TLS/443: pass/fail + message
Backstage: pass/fail
Audio and video in both directions for two minutes: pass/fail
Stage entry: pass/fail
Reconnection when data returns: pass/fail
Automatic selection without choosing a relay: pass/fail
```

Do not record phone numbers, IMSI/IMEI, IP, passwords, invitation keys or TURN credentials. The diagnostic does not send these results to a third party and does not identify the operator automatically. The operator is recorded from the line used by the person running the test.

## Complementary automated tests

CI checks negotiations with late ICE, failure and retry, per-participant isolation between routes, and video through TURN/TCP. The manual `relay-connectivity` workflow lets you choose `mode=tls` or `mode=local` against an installation reachable from the internet. The local test can be run while Cloudflare stays active for other participants. The browser tests also simulate old JavaScript in the cache and require the new diagnostic to keep receiving video.

These tests detect software and transport regressions, but they do not simulate the APNs, NAT, DNS or policies of every carrier. Only mark a row as confirmed when you obtain evidence from that real network.
