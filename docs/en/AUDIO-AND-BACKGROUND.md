# Room audio, screens, and Chrome on Android (1.11.4)

[Español](../AUDIO-Y-SEGUNDO-PLANO.md) · **English** · [Русский](../ru/AUDIO-AND-BACKGROUND.md)

## Participant reconnection

Each publication and reception has its own negotiation. A delayed close or answer cannot remove its replacement even if it keeps the same camera identifier. ICE candidates and answers from known negotiations that have already ended are discarded for a limited period, without showing errors when other participants join. Unknown identifiers or ones belonging to another socket are still rejected.

Only the actual closing of the WebSocket causes the session to be recovered automatically, preserving admission. There is no button or reconnection notice during the call. Ping, a signal drop, transient ICE states and going to the background do not restart publications, subscriptions or the room. Explicit provider changes by the host and quota protection still apply. If a new track from a source appears, only its receiver is updated.

## Switching tabs or apps

Participant sound plays in audio elements separate from the video views. Pausing or hiding a video does not mute the room. On return, playback is retried, audio contexts are resumed and captures that Android has ended are recovered, without changing an active socket or one in the process of joining. Cameras or microphones disabled by the user stay disabled. If Android ends the host's camera, its microphone and publication are preserved; the camera can be recovered on return, without removing the host from the room.

In the call, **Floating window** opens the enlarged camera, or the first available remote camera, through Picture-in-Picture when the browser has that API. You must press the button; a website cannot open that window arbitrarily. If Chrome blocked audio, **Enable room audio** lets you resume it with one tap, without permanently muting videos as an alternative.

This preserves connections as long as Chrome and Android allow them to run. It does not force the system to keep a discarded tab, a suspended camera, a closed browser or a phone under battery restrictions. Picture-in-Picture availability depends on the version and the device. Hidden-tab tests verify the web logic and the audio flow; they do not certify every physical phone.

The Android APK offers **Background** after granting microphone access and entering the room. It keeps the page alive through an audio service started with the app visible and a permanent notification; it does not discard the page when using other apps. The notification lets you return or turn the mode off. Closing the app from recent tasks stops it. The camera remains subject to Android restrictions. The iOS app keeps its foreground behavior. See [installation and details](../../mobile/README.md).

## Sharing sound from a screen

In desktop Chrome, choose a **tab** and enable **Share tab audio** in the browser picker. Sharing only a window does not guarantee capturing its sound. System audio capture depends on the browser and the operating system; StreamWebRom cannot enable a permission or a checkbox for the user.

The program prefers sharing **another tab with isolated audio**, excluding the room itself and the mix of the entire system. It keeps the content's sound without echo cancellation or noise suppression. If the browser rejects advanced options, it requests audio again and checks the chosen surface: a general capture without a confirmed `restrictOwnAudio` filter continues **video only**, with a notice explaining how to share sound without sending the voices back.

A mix that already contains the room voices cannot be separated correctly for each participant. That is why that general audio is not re-broadcast as if it were independent content. Where Capture Handle is available, a StreamWebRom tab is also identified, even if the browser ignores the initial exclusion. For the shared content to reach the program, the host must add the screen to the stage and keep its volume active.

## Platform references

- [Page lifecycle in Chrome](https://developer.chrome.com/docs/web-platform/page-lifecycle-api): a tab can be frozen or discarded and JavaScript cannot prevent it in general.
- [Audio Session API](https://developer.mozilla.org/en-US/docs/Web/API/AudioSession): `play-and-record` declares a call session where available; it is an API with limited compatibility.
- [Screen capture in Chrome](https://developer.chrome.com/blog/avoiding-oversharing-when-screen-sharing): surface selection and audio controls.
- [Screen capture specification](https://www.w3.org/TR/screen-capture/): `restrictOwnAudio`, `systemAudio`, exclusion of the own tab and filter behavior.
