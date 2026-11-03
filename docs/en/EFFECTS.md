# Low-latency camera effects (1.9.0)

[Español](../EFECTOS.md) · **English**

Camera settings keep normal/strong background blur, **whole-video blur** (no segmentation: the entire frame is softened), **pixelation** (blocks as large as the user wants), color or image/video backgrounds and camera replacement. Both whole-frame filters carry a **Strength** slider from 0 to 100 that applies live, without restarting the camera; they can also be fixed from the URL with `&effects=pixelate` (or `fill-blur`) and `&strength=60`. Under **Effect performance** there are three profiles: **Low latency**, up to 720p with a mask at 12 FPS; **Balanced**, up to 1080p with a mask at 20 FPS; and **More detail**, up to 2160p with a mask at 30 FPS. The camera composites recent frames at the available rate without waiting for a new mask on each one. The caps are processing limits, not a guarantee of the hardware's FPS or resolution.

Person segmentation runs in a separate worker, with the GPU when available and the CPU as a fallback. Only one pending bitmap is allowed per camera: subsequent ones are discarded while the engine is busy. The worker returns a small mask. Frames are processed on the device; the models are bundled and are not loaded from a CDN. When saving a persistent room, the host's small background images present in their settings are also saved on their server as private options. Local video files must be selected again.

The horizontal call model SelfieSegmenter landscape (144×256, float16) is used in the light profiles where appropriate; in portrait or with more detail the square model is kept. Discord's proprietary engine is not used. The Discord research serves as a functional reference for blur, backgrounds and preview.

If the mask takes too long, the full background or the whole-image blur is kept while a recent one arrives; camera frames are not accumulated. If the worker cannot start, a warning is shown and the normal camera is restored. Disabling the effect or stopping the camera releases the worker, the captures, the clock and the auxiliary videos.

For the lowest possible load, use **No effect**. If you already process the camera in OBS, select **OBS Virtual Camera** and leave No effect in StreamWebRom to avoid applying two processings. OBS offers color filters and chroma key; chroma key requires a suitable physical background. OBS's native background removal plugins cannot be loaded directly as web filters.

## References reviewed

- [Discord: Video Backgrounds](https://support.discord.com/hc/en-us/articles/4413490191127-Video-Backgrounds): blur, preset and custom backgrounds, and preview.
- [Google: web segmentation](https://developers.google.com/edge/mediapipe/solutions/vision/image_segmenter/web_js): synchronous inference blocks the interface; it recommends workers.
- [Google: segmentation models](https://developers.google.com/edge/mediapipe/solutions/vision/image_segmenter/index#models): square and landscape versions for segmenting people.
- [Chrome: video processing](https://developer.chrome.com/docs/web-platform/best-practices/webcodecs): processing off the interface and transferable frame resources.
- [OBS: filters](https://obsproject.com/kb/filters-guide) and [chroma key](https://obsproject.com/kb/chroma-key-filter).
- [OBS Background Removal plugin](https://github.com/royshil/obs-backgroundremoval), a native implementation different from the web client.

The validation compares the previous version and this one on the same machine with generated video, recording rendered frames, main thread blocking and resource release. It does not certify a universal millisecond reduction or performance equal to Discord's native client.
