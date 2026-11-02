# Connection, quota and the VDO.Ninja alternative (v1.11.4)

[Español](../CONEXION-Y-VDO.md) · **English**

When you create a studio, choose **StreamWebRom · local relay** or **VDO.Ninja · standalone room**. They are different rooms: creating a VDO.Ninja room does not create a room in the local SFU, nor does it use the StreamWebRom server's Cloudflare credentials.

## StreamWebRom rooms

All new rooms use the local relay. In **View → Connection & savings**, the host can select Cloudflare only if the room was created with a password and validates the server's master password. Verification happens in the backend, with bcrypt and an attempt limit; modifying the JavaScript or a status message cannot bypass it. Returning to the local relay does not require the master password.

Cloudflare credentials do not appear in `/api/config`. They are only handed out after authenticating a previously authorised room, including automated guest tests and OBS/WHIP/WHEP outputs. The master password is never sent to guests or stored in the browser settings. On Windows the hash can be kept encrypted with DPAPI in `CloudflareMasterHashProtected`, inside the relay's private configuration; the launcher passes it to the server through `SWR_CF_MASTER_HASH`. Do not publish that file.

For other installations, `streamwebrom hash-master` reads the password from standard input and writes its bcrypt hash; use a hidden prompt and store only the hash in the service's private configuration. Do not pass passwords as process arguments. With no hash configured, Cloudflare activation stays blocked.

The connection choice is kept when you save a persistent room. Changing the relay reconnects only that room, keeping its source identifiers. The quota cut-off affects only rooms authorised for Cloudflare; local rooms stay connected.

## Shared quota and data saving

Cloudflare advertises 1,000 GB per month free, shared between TURN and SFU; they are not two independent quotas. The control reads both sets of statistics and compares their total against a conservative traffic estimate. The default safeguard cut-off is **900 GB**, with a warning from **800 GB**. The configuration does not allow raising the cut-off above 900 GB.

When the safeguard cut-off is reached, hosts of the affected rooms see a notice and a **15-second** countdown before the automatic switch to the local relay. If usage cannot be verified, or the free allowance has already been used up, the switch is immediate. The block is stored for the month and is not reset by restarting the program. If the registry is damaged, the original is kept and Cloudflare stays paused until a reliable record is recovered. A recovery reserve is shown separately from verified usage.

This mechanism is preventive: statistics may lag and other services on the account share the allowance. It is not equivalent to a billing limit imposed by Cloudflare.

Send profiles the host can change during the call:

| Profile | Cameras | Shared screen | Mono / stereo audio |
| --- | --- | --- | --- |
| Balanced (default) | Up to 720p, 24 fps, 1,500 kbps | Up to 1080p, 15 fps, 2,500 kbps | 64 / 128 kbps |
| Maximum savings | Up to 540p, 20 fps, 900 kbps | Up to 1080p, 8 fps, 1,500 kbps | 48 / 96 kbps |
| Original quality | Settings chosen by each source | Original settings | Original settings |

These are upper limits; lower requests and the camera's capability are respected. The 16:9 framing and the resolution chosen for the program/OBS do not change. OBS receives only the sources in its scene, along with the audio of sources whose video is hidden, and keeps the individual links outside the stage. Traffic that uses the local route is not charged to the estimated Cloudflare meter.

## VDO.Ninja standalone room

The room opens at `/vdo` with an independent random identity. The visual frame keeps the themes, the bar, the collapsible panel, the invitation/OBS shortcuts and the full-screen 16:9 stage view. Participant controls and the composition inside the frames belong to the official VDO.Ninja application. This option does not automatically replicate StreamWebRom branding, multistream, recording or persistent storage.

The integration loads `https://vdo.ninja/` in iframes through its documented mechanism. It does not distribute VDO.Ninja code, obtain its TURN credentials or use its private endpoints with a client of its own. The service keeps its own terms, privacy and availability. Host and guest links are different; the guest and OBS do not receive the co-director key. Passwords and the director key travel in the `#` fragment, which the browser does not send in the HTTP request. If a platform strips the fragment when sharing, use the original full link.

The history of these rooms is kept in the **browser session**; use **Save host link** to keep a private copy and return to the same room. A VDO.Ninja password accepts up to 49 letters/numbers. If you do not type one, a random password is generated for the links. The room is initialised on the official service when its director enters; creating the link does not reserve a dedicated server.

In mobile apps, only the official domain can load this external frame. Camera/microphone permissions are requested again and are granted only to the VDO.Ninja frame, from the `/vdo` screen. If the WebView does not support a feature, open the official app in Chrome/Safari from the available button. Not all physical devices have been certified.

You can also keep the full StreamWebRom studio and add an external camera from **VDO.Ninja** in the bottom bar: generate an invitation or connect a viewing link. The viewer offers a direct link for OBS; on a computer its tab can be captured as a regular source for the backstage. Capturing requires choosing the tab and sharing its audio, adds processing and is not available on all phones. The iframe's video is not copied automatically into the compositor because of browser origin restrictions.

## Backstage without overlapping controls

Each thumbnail keeps two actions: **Stage/Remove** and **Options**. Tapping the camera or opening Options shows an enlarged 16:9 view with separate buttons for audio, video, pin, name, OBS, quality, volume and moderation. Closing the panel releases its extra playback; hiding the video also hides this enlarged view. It does not change the guest's microphone state.

## Free alternatives reviewed on 20 October 2026

| Alternative | What is free | Considerations for Chile |
| --- | --- | --- |
| [Pion TURN](https://github.com/pion/turn) | MIT software, already integrated as the local relay | The machine and its Internet access provide the capacity. The current public route through Funnel may add latency. |
| [Coturn](https://github.com/coturn/coturn) | Free TURN/STUN software to deploy | Suitable for a standalone server with public access and TLS; changing software does not guarantee a better network route. |
| [Cloudflare Realtime](https://developers.cloudflare.com/realtime/sfu/platform/pricing/) | 1,000 GB/month shared SFU/TURN | Option already supported, with protected activation and a safeguard cut-off. |
| [ExpressTURN](https://www.expressturn.com/) | Advertises 1,000 GB/month, TCP/UDP 3478 on the free plan | Do not assume 443/TLS or region selection on the free plan; it requires an account and a real test. |
| [Open Relay](https://www.metered.ca/tools/openrelay/) | Advertises 20 GB/month and 443/TLS | [Metered's commercial page](https://www.metered.ca/pricing/) advertises a different 500 MB trial and [its free regions](https://www.metered.ca/docs/turnserver-guides/turnserver-regions/) are limited. Confirm what the account is assigned before integrating. |
| [VDO.Ninja service](https://docs.vdo.ninja/guides/iframe-api-documentation) | Hosted application with its own connection infrastructure | It is included as an official external application. Its software licence does not grant independent access to its relays. |

No accounts have been created and no ExpressTURN/Metered credentials added. No new provider is enabled automatically in local rooms. Choosing by speed requires tests with decoded video and real RTT from Movistar, Entel, Claro and WOM; advertising geographic proximity does not prove latency on those networks.

Integration sources: [licence and service](https://github.com/steveseguin/vdo.ninja/blob/develop/LICENCE.md), [iframe API](https://docs.vdo.ninja/guides/iframe-api-documentation), [private fragments](https://docs.vdo.ninja/help/privacy-and-security-details/fragment-url-parameters), [VDO.Ninja terms](https://docs.vdo.ninja/help/privacy-and-security-details/vdo.ninja-terms-of-service).

Automated tests: backend permissions and master password, shared quota and the warning before the cut-off, real send profiles, OBS subscriptions per scene and the enlarged panel without overlap at five sizes. The VDO.Ninja flow uses a simulated iframe in the interface regression; that test does not certify a real call over its infrastructure or its latency.

### Room identifiers

Since v1.11.1 VDO IDs have at most 30 alphanumeric characters. 128 bits of randomness are kept, encoded in base 36; up to five letters or numbers from the optional prefix are used. The program title keeps its full name. Long v1.11.0 links are converted to the same ID for director, guest and scene; copy the OBS link again from the updated room if you had copied the direct link from the previous service.

### Recovery and relay changes

Since v1.11.2 a temporary WebRTC state does not request a reconnect or close the connection. The notice requires ten seconds without reception or link responses. If packets reappear, it is withdrawn automatically. Changing provider keeps the host's captured camera and microphone, the IDs and the additional cameras; guests keep the sources that arrive during their recovery. An earlier automatic choice of the local relay re-evaluates the room's preference, while a manual choice is kept.
