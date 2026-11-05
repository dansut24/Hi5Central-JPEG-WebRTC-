# Hi5Central JPEG-over-WebRTC — Portable Device Portal

Independent user-mode Windows screen-sharing prototype; not linked to the Hi5Central ITSM/RMM Agent.

## One-click Windows sharing

Download the latest green GitHub Actions build and extract `Hi5-JPEG-ScreenShare.exe`.
Double-click as a normal Windows user, **no administrator rights, installation, service or driver required**.
The EXE is preconfigured to connect to `https://jpeg.hi5central.com` and registers the current Windows computer name automatically.
It visibly stays open and can be closed at any time to stop sharing.

Open **https://jpeg.hi5central.com/portal** in any browser, sign in with the portal credentials configured on the IONOS server, and select the online computer.
The WebRTC connection and JPEG stream start **only when a viewer clicks View**. The application never captures or transmits frames while merely waiting for a viewer.

## Architecture

Portable Windows GDI JPEG capture → encrypted WebRTC DataChannel (P2P or existing IONOS TURN fallback) → browser canvas. The small Go signalling server runs on the existing IONOS VPS behind Caddy wildcard TLS. No public port is exposed by the new Docker container.

The portal lists currently online devices and allows one viewer per device. Viewer access requires HTTP Basic authentication over HTTPS and creates a short-lived, one-device viewing capability. Tokens never appear in the public device list; room names are sanitized. No remote keyboard/mouse commands or unattended access are supported. Screen sharing is an explicit action (launching the visible EXE) and stops on exit.

The original private session-link mechanism remains available for testing but its initial connection window expires after 10 minutes. The portal can create new short-lived viewing links for any active computer.

## Build

GitHub Actions compiles the portable Windows EXE and Linux binaries, runs Go tests and validates browser JavaScript. Use the `hi5-jpeg-webrtc-standalone` artifact from the latest successful workflow run.

You can override defaults, for example:
```
Hi5-JPEG-ScreenShare.exe -server https://jpeg.hi5central.com -device "My PC" -fps 20 -quality 75 -width 1920
```

## Deploy

See [deploy/ionos/README.md](deploy/ionos/README.md). Set `PORTAL_PASSWORD` (at least 16 characters) in the relay's env file; optional `PORTAL_USER` defaults to `hi5`. Portal access is disabled if no valid password is configured.

**Experimental proof of concept only.** Current shared portal login is suitable for limited internal testing, not a production multi-tenant service. Expand to tenant-specific authentication, rate limiting, identity mapping, audit logs and signed EXEs before broad release.

## Windows per-user installer and tray

The latest successful GitHub Actions ZIP includes `Hi5-JPEG-ScreenShare-GUI.exe` and the `scripts/` folder. Extract the entire ZIP, then double-click `scripts/Install-User.cmd` and type **INSTALL** to consent to unattended viewing of the signed-in desktop. The app copies to the installing user's `%LOCALAPPDATA%\\Hi5Central\\JPEGWebRTC`, starts a visible notification-area icon, and registers a **current-user-only** Windows Run key to start at each login. It needs no elevation. You can pause/resume or exit from the icon at any time, and use `scripts/uninstall-user.ps1` to remove auto-start.

The background GUI executable is built with the Windows GUI subsystem (no console) and writes diagnostics to `%LOCALAPPDATA%\\Hi5Central\\JPEGWebRTC\\agent.log`. The icon clearly indicates when remote screen viewing is active. The user-mode agent does not run at the Windows login screen or before any user has logged in. It reconnects while the tray is open after transient failures. Only deploy on computers with informed consent from the desktop user and authorisation for attended/unattended support.
