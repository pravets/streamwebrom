# Sharing windows, tabs, and audio (v1.12.0)

[Español](../CAPTURA-AVANZADA.md) · **English** · [Русский](../ru/ADVANCED-CAPTURE.md)

The studio and the guest controls keep their interface. The share button lets you choose the source and the sound. The host still authorizes guests before they share.

## Advanced capture for Windows x64

Download `StreamWebRom-Capture-Windows-x64.zip` from the project releases. Extract the entire folder and run `StreamWebRom-Capture.exe`. Open your HTTPS server or `http://localhost:puerto` and enter the usual room. You do not need to create another server or use another relay.

The launcher offers "Advanced capture". If you install the contents of the ZIP folder in `%LOCALAPPDATA%\StreamWebRomCapture`, that button opens the client with the configured server; if it is not installed, it opens the downloads.

- **Window:** shares video of the window and audio of the chosen application. Several windows of the same program may belong to the same process and share its sound.
- **Full screen:** shares the monitor and system audio, excluding the process tree of the client that plays the room voices.
- **Web tab:** opens a content address inside the client and selects that tab. Its audio is captured independently. Tabs already open in Chrome are shared from Chrome, or you can open their address in the client.
- **No audio:** transmits video only.

Use the client also to listen to the room. If you play the call in another program outside the client, that playback belongs to the system audio and cannot be excluded automatically. Do not open a second audible copy of the call while sharing the system. Content tabs opened inside the client are shared with "Web tab"; system mode excludes their entire tree to protect the voices.

Native capture uses WASAPI Process Loopback. It requires a version of Windows that supports that API and a working audio device. If Windows does not allow it, the picker shows the error and lets you choose another source or video without sound. This native client is distributed for Windows x64; equivalent native capture is not advertised on Android, iOS, macOS or Linux.

## From Chrome and other browsers

The picker offers tab, window or full screen. The browser decides which sources and audio it allows on that operating system. For audio from another tab, enable "Share audio" in the browser dialog. General audio is kept only when the browser confirms it can exclude the call itself; if it cannot isolate it, video is sent and the user is told to use Advanced capture. This check prevents sending the participants' voices back.

## Implementation and validation

The Windows engine adapts the MIT component [electron-native-screenshare](https://github.com/CilginSinek/electron-native-screenshare), pinned to the commit indicated in `desktop/vendor/native-audio/ORIGIN.md`. The license and provenance travel with the ZIP. The capture uses a helper process with a startup deadline and bounded buffers; a device failure stops that source, without restarting the room. The site receives no Node APIs or free access to processes.

Physical Windows tests use generated tones to distinguish the call, an external application and a tab. They verify per-application inclusion, exclusion of the Chromium tree that plays the call, tab audio and PCM transport through the helper process. They do not save recordings. CI checks the real picker, video publication, content isolation and the packaged client. These tests do not certify every available driver or device.

## Building and publishing

`npm ci --ignore-scripts`, `node node_modules/electron/install.js`, `npm run native:build`, `npm test` and `npm run package:windows`, inside `desktop`, with Node 24 and Windows C++/SDK tools. Versions are pinned in the lockfile. Server, mobile and client artifacts are published together after their checks pass and `SHA256SUMS.txt` is computed. Tag jobs do not overwrite published files.
