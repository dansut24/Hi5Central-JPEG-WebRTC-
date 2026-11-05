# Hi5Central JPEG-over-WebRTC — Portable Device Portal

Independent user-mode Windows screen-sharing prototype; not linked to the Hi5Central ITSM/RMM Agent.

## One-click Windows sharing

Download and extract the latest successful GitHub Actions ZIP, then double-click **`scripts/Install-User.cmd`**. There is **no INSTALL prompt** and no administrator elevation. The script stages the background EXE in a unique directory under `%TEMP%`, then copies it to the current user's `%LOCALAPPDATA%\Hi5Central\JPEGWebRTC`, registers current-user sign-in startup and starts a **visible system-tray icon**. The EXE connects automatically to `https://jpeg.hi5central.com` and registers the Windows computer name. The local user can Pause or Exit from the tray at any time.

Open **https://jpeg.hi5central.com/portal** in any browser, sign in with the portal credentials configured on the IONOS server, and select the online computer.
The WebRTC connection and JPEG stream start **only when a viewer clicks View**. The application never captures or transmits frames while merely waiting for a viewer.

## Architecture

Portable Windows GDI JPEG capture → encrypted WebRTC DataChannel (P2P or existing IONOS TURN fallback) → browser canvas. The small Go signalling server runs on the existing IONOS VPS behind Caddy wildcard TLS. No public port is exposed by the new Docker container.

The portal lists currently online devices and allows one viewer per device. Viewer access requires HTTP Basic authentication over HTTPS and creates a short-lived, one-device viewing capability. Tokens never appear in the public device list; room names are sanitized. This is unattended, **view-only** support for the signed-in user's desktop: a portal operator can start viewing without an additional approval prompt after installation. No remote keyboard/mouse commands are supported. The local user retains visible sharing status and Pause/Exit controls.

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

The ZIP contains `Hi5-JPEG-ScreenShare-GUI.exe` and a `scripts/` folder.

1. Extract the complete ZIP on Windows.
2. Double-click `scripts/Install-User.cmd`. It stages the EXE and scripts in a temporary folder, copies them into the installing user's `%LOCALAPPDATA%\\Hi5Central\\JPEGWebRTC`, and starts the tray app without an `INSTALL` prompt or elevation.
3. The agent registers itself under the Windows computer name. Sign into the protected portal at `https://jpeg.hi5central.com/portal` to see online computers and click **Start viewing**.
4. The tray icon indicates ready/streaming state. The local user can Pause, Resume, or Exit sharing. The operator can stop the active viewer from the portal.
5. To remove auto-start and files, run the installed `uninstall-user.ps1` from the user profile folder.

The background EXE has no console and writes diagnostics to `agent.log` in the same directory. It launches on Windows sign-in for the installing user. This user-mode agent **does not run at the Windows sign-in screen**, and a computer without a signed-in user cannot be streamed. Deployment requires organisational authorisation and informed consent. The test portal needs its protected IONOS service activated before portal viewing can work.
