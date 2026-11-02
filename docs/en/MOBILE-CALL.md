# Calls from the phone

[Español](../LLAMADA-MOVIL.md) · **English**

Since 1.7.0, videos regroup automatically according to the number of visible sources and the available space, both in portrait and landscape. The control bar and the notices have their own space.

Since 1.8.0, all tiles keep a **16:9 landscape** frame, even on a portrait phone and when enlarging a participant. Cameras use a centered crop that fills the tile without stretching the image, as in the host's studio. A portrait camera loses its top and bottom edges in that crop: place the face in the center or rotate the phone before entering. Shared screens are fitted entirely inside the 16:9 frame so no text is hidden.

OBS outputs also keep a 16:9 frame. Configure the browser source at **1920 × 1080**; the program preserves exactly the host's compositor. Individual camera sources use that same crop by default, even with old links. If the full camera needs to be shown, uncheck "Studio framing (16:9)" when copying the link: `cover=0` keeps the full image inside the frame, with letterboxing where applicable.

Tap a video to see it larger. The others remain as thumbnails. Tap the enlarged video again or press "View all" to restore the grid. If the enlarged source goes away, the grid returns automatically. This selection only changes your screen; it does not change the host's program or stage. The volume control does not change the selection.

Brief interruptions have ten seconds to recover before reconnection is offered. While the signaling recovers, an informational status may appear without asking you to act. If WebRTC confirms a definitive failure, it is shown immediately. An initial failure offers "Retry entry", and a real loss after connecting offers "Reconnect".

Relay selection remains automatic on entry. For 4G/5G networks it is still necessary to configure the server and TURN; see [mobile networks](MOBILE-NETWORKS.md).

On exit, the camera, microphone, shared screen and additional cameras are stopped. "Join again" reopens the entry screen keeping the invitation.

## Sharing the screen with authorization

When you are on the program's stage or another active scene, press the screen icon to **request authorization**. The host sees a request over the backstage and can approve or reject it. After approval, **Share screen** appears: press it to choose the window or screen in the system picker. Nothing is captured or published before that click.

The host receives the screen as an independent source and chooses when to add it to the stage. They can stop it with **Revoke screen** under People. Removing the guest from all stages, disconnecting or stopping the capture ends the permission; a new one is requested to share again. The camera and microphone remain independent.

`getDisplayMedia` is required in the browser, along with the system capture permission. Mobile browsers and WebViews that do not offer that API show the control disabled; use a compatible desktop browser to share. This does not limit joining from the phone or viewing a screen shared by another participant.

## Since 1.9.0

"Latencies" shows or hides RTT and battery on each tile. RTT is the round trip to the media server; "Room" measures this device's signaling ping. They are different measurements from the total delay between camera and screen. They are polled every three seconds and stale data is discarded. The battery and charging icon only appear with a value when the browser provides that API; "—" indicates data unavailable.

Press **Reorder** and use the arrows on each tile. The order is local and is preserved when rotating the device; it does not alter the host's stage. "Finish ordering" hides the arrows. All tiles keep 16:9. In landscape, when enlarging a video, the thumbnails move to the side if that yields a larger image.

**Full screen** uses the whole window and requests native fullscreen when available. Controls appear when tapping or moving the pointer and hide when unused. You can always exit from the top button or Escape. A screen with a different aspect ratio may leave space around the group to preserve 16:9 without distorting the videos.
