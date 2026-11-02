# Studio, quality, and OBS outputs (1.9.0)

[Español](../ESTUDIO-Y-OBS.md) · **English** · [Русский](../ru/STUDIO-AND-OBS.md)

In the studio, **Stage only** opens an additional view of the program without backstage, menus or production controls. The video fills the largest available 16:9 rectangle. Move the pointer or tap to see "Back to studio" and "Full screen". It does not change the production or disconnect the room. Escape returns.

**Latencies** shows each participant's RTT and their battery/charge if they can be queried. "Room" is the ping of your signaling connection, not an average of all participants. The RTT to the SFU is not equivalent to the total latency of capture, effects, encoding and playback. Only numeric information is published; no IP addresses or ICE candidates are shared. Battery data is not available in all browsers.

## Camera send quality

In the controls of each backstage camera, **Send quality** appears. The selection adjusts the publisher's requested resolution, FPS and bitrate, including an additional camera. It applies to all receivers of that source. The hardware and the browser may deliver a different resolution than requested. The apply notice reports the size obtained.

Full camera settings keep resolution, FPS, codec and processing options. Effects in low-latency mode limit their canvas to 720p: select a higher effects profile if you need to send more detail.

## OBS resolution and FPS

The OBS panel offers 640×360, 960×540, 1280×720, 1920×1080, 2560×1440 and 3840×2160, with 15/24/25/30/50/60 FPS. Copy the link after choosing and configure OBS's Browser source with the same dimensions. The parameters are `quality=360|540|720|1080|1440|2160` and `fps=15|24|25|30|50|60`.

These options define the browser's output canvas and do not increase the detail that arrives or by themselves reduce the source's bandwidth. To reduce data, use Send quality. The SFU forwards the camera: it does not transcode a different version per receiver. The program and the scenes keep the host's proportions and composition when the canvas size changes.

Cameras keep a centered 16:9 crop; `cover=0` keeps the full content inside the frame. Shared screens are shown complete by default. Old links keep working.

## Startup and recovery

When the receiver connects, the server requests a keyframe again: the initial request could be lost before completing ICE/DTLS. Individual OBS sources use a single decoder. A failed media connection or one without first frames is reopened; if configured, the local relay can be tried. A signaling connection that reopens also restores the audio mix.

Transient media and signaling interruptions wait ten seconds before offering reconnection. A definitively failed WebRTC state can start recovery immediately.

## Saving the production

Rooms with a password can be saved with an expiration from the Save room button in the top bar. See [Persistent rooms](PERSISTENT-ROOMS.md). Hiding a camera also hides its image in backstage and keeps the audio.
