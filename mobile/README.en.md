# Mobile clients 1.12.0

The mobile connection fix includes server and web changes: update the server to 1.12.0 as well. Networks with CGNAT/blocked UDP need a configured TURN service; installing a new APK by itself does not provide a relay. Chrome keeps active connections when switching apps as long as Android allows it to run. The Android APK also offers an explicit background audio mode with a service and a notification. See [mobile networks](../docs/en/MOBILE-NETWORKS.md) and [audio and background](../docs/en/AUDIO-AND-BACKGROUND.md).

Android and iOS open the studio served by your StreamWebRom installation using the system's WebRTC. They do not run the SFU, ffmpeg or the recordings on the phone. Use HTTPS with a valid certificate; self-signed certificates without trust are not accepted.

## Android

Android 8 or later, with an up-to-date Android System WebView. Enter the HTTPS URL of your server or an invitation link. Permissions are requested when the page uses the camera or microphone and are granted only for the selected origin. External links open in the browser. The file picker lets you choose images or videos without general storage permissions.

To use other apps during the call, enter the room, grant the microphone permission and press **Background** in the native bar. Android 13+ requests permission for the notification. The service starts while the app is visible; it does not start after a reboot or after the system kills the process. In this mode the page, the audio and the connection are kept when switching apps, and a notification lets you return or disable the mode. The service does not record audio by itself. Android may pause the camera or impose battery restrictions; this does not certify every physical model. Without the mode enabled, the app releases the page when hidden. Closing the app from recent tasks stops the service.

Build with JDK 17+, Android SDK 35 and Gradle 8.11.1:

```sh
gradle -p mobile/android :app:assembleDebug :app:assembleRelease :app:lint
```

On Windows there is also a direct build, without the Gradle daemon:

```powershell
$env:JAVA_HOME = 'C:\ruta\a\jdk17'
$env:ANDROID_HOME = "$env:LOCALAPPDATA\Android\Sdk"
./scripts/build-android.ps1
```

- `dist/StreamWebRom-Android-debug.apk`: installable for tests. The debug key stays outside the repository.
- `dist/StreamWebRom-Android-unsigned.apk`: requires signing with your distribution key before installing/publishing.

For Google Play, generate an AAB with `gradle -p mobile/android :app:bundleRelease`, configure it with your signature and complete the store requirements. The delivery does not include publishing to Google Play.

## iPhone / iPad

iOS/iPadOS 16 or later. On macOS, install [XcodeGen](https://github.com/yonaskolb/XcodeGen), and run:

```sh
cd mobile/ios
xcodegen generate
xcodebuild -project StreamWebRom.xcodeproj -scheme StreamWebRom -sdk iphonesimulator -configuration Release -derivedDataPath build CODE_SIGNING_ALLOWED=NO build
```

The GitHub `mobile` workflow performs this build and keeps a ZIP for the simulator. **It is not an IPA installable on an iPhone.** For a physical device, open the project in Xcode, select your development team and configure signing and profiles. Complete the icons and Apple's privacy requirements before distributing on the App Store/TestFlight.

WKWebView allows camera/microphone only for the chosen origin and asks for consent. TLS errors are not ignored. The session's web data uses non-persistent storage; the native preferences store only the server origin, without host keys or full links. Leaving the foreground releases the page; returning reloads it.

The iOS application remains foreground-only; distributing to physical devices requires an Apple signature. Native screen sharing and PiP are out of scope for this version. Android's Browser button and the PWA offer alternatives depending on the browser's capabilities. The PWA remains available on both platforms without installing binaries.
