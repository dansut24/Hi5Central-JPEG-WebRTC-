# Hi5Central JPEG-over-WebRTC

Standalone **view-only** Windows screen-sharing experiment, separate from the main Hi5Central RMM agent/viewer.

- **Portable Windows EXE:** ordinary user-mode desktop capture with GDI, JPEG encoding, encrypted WebRTC DataChannel; no administrator rights, installation, service or driver.
- **Browser viewer:** built-in HTML canvas with JPEG reconstruction and FPS/bitrate counter.
- **IONOS VPS relay:** small Go signalling process behind the existing Hi5Central Caddy HTTPS gateway. Direct WebRTC is preferred; the existing coturn service can optionally relay traffic if direct ICE fails.
- **GitHub Actions:** Windows portable EXE and Linux AMD64/ARM64 binaries.

The standalone viewing URL is created by the sender and should only be shared with an intended viewer. The sender must be running and a normal interactive desktop must be available. Viewer is **view-only** (no mouse/keyboard input).

**Experimental proof of concept, not production-hardened.** Use consented test sessions only. Links expire for new connections after 10 minutes, but established sessions persist while the sender stays connected. Screen sharing stops when the sender exits. Stronger authentication, rate limiting, logging and signing would be required for a wider release.

## Build

GitHub Actions builds on push. Download `hi5-jpeg-webrtc-standalone` under the latest green Actions workflow.

Run `Hi5-JPEG-ScreenShare.exe -server https://jpeg.hi5central.com` as an ordinary Windows user. A browser viewing URL is printed in the application.

For the IONOS deployment see [deploy/ionos/README.md](deploy/ionos/README.md). The server is optional for local builds but required for internet sessions.
