# Oracle Cloud Always Free deployment

The Oracle VM hosts WebSocket signalling and the HTTPS browser viewer.
JPEG data goes peer-to-peer over WebRTC when possible; TURN fallback relays media via the VM.
The Windows application does not require an installer, service or administrator permissions.

## Preparation

Use a NEW Oracle VM or dedicated service. Never overwrite the Hi5Central RMM Caddy/TURN configuration.
Point an HTTPS hostname to the Oracle VM, and permit TCP 80/443 through both OCI VCN and OS firewall.
Keep TCP 8787 private (loopback only).

## Install

Download the successful GitHub Actions build artifact. Identify VM architecture with `uname -m`:
aarch64 = Linux arm64; x86_64 = Linux amd64.

```bash
sudo install -d -m 0755 /opt/jpeg-webrtc/web
sudo install -m 0644 web/index.html /opt/jpeg-webrtc/web/index.html
sudo install -m 0755 jpeg-relay-linux-arm64 /usr/local/bin/hi5-jpeg-relay
# Replace arm64 with amd64 for an AMD/x86_64 VM.
sudo install -m 0644 deploy/oracle/hi5-jpeg-relay.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now hi5-jpeg-relay
curl -I http://127.0.0.1:8787/
```

Add the example dedicated host to Caddy without replacing existing sites.
Validate and reload Caddy. HTTPS must be working before public browser viewing.

## Optional TURN

Some firewalls prevent direct ICE. Install coturn with `use-auth-secret`,
put a random `static-auth-secret` in the coturn configuration, and create
`/etc/hi5-jpeg-relay.env` with:

```text
STUN_URL=stun:share.example.com:3478
TURN_URL=turn:share.example.com:3478?transport=udp
TURN_SECRET=<same-coturn-secret>
```

Limit that env file to root (0600). Open UDP 3478 and a tightly scoped
relay port range in OCI VCN and OS firewall. Set `external-ip` correctly
if Oracle NAT applies. Optional TURN/TLS over TCP improves corporate reachability.

## Windows

Run `Hi5-JPEG-ScreenShare.exe -server https://share.example.com` as a regular user.
The EXE prints a private, short-lived viewing URL. Open it on another device.
Close the EXE to stop. Do not share links publicly.

## Status

Source proof of concept only. Do not use for unattended access or production
without security review, rate limits, abuse prevention and executable signing.
