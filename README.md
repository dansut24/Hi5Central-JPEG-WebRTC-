# Hi5Central JPEG-over-WebRTC

Standalone **view-only** screen-sharing prototype. Separate from Hi5Central RMM and ITSM.

A portable Windows user-mode EXE captures the logged-in desktop and JPEG-encodes frames. JPEG chunks are carried over an encrypted WebRTC DataChannel, then displayed by a browser canvas. A small Go signalling relay issues short-lived viewing links and can issue temporary coturn TURN credentials.

No admin rights, installation, service, driver or inbound Windows port required on the Windows side. The EXE must remain running and visible to share. No remote control or unattended access.

This is an experimental build, **not production-hardened**. Do not expose raw HTTP. Deploy the signalling relay behind HTTPS and provide access to consented users only.

Use the GitHub Actions workflow to build a Windows portable EXE and Linux relay for x64 and Oracle Ampere ARM64.

See `deploy/oracle/README.md` for deployment instructions.