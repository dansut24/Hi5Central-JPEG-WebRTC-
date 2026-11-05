# Deployment to existing IONOS Hi5Central VPS

This uses the existing VPS running `hi5central-gateway-1` (Caddy wildcard TLS) and `hi5central-turn-1` (coturn).
**Do not restart, replace or update the Hi5Central gateway, TURN service, RMM API, or their Compose file.**

The new relay is an independent Docker container on the existing `hi5central-internal` Docker network.
It does not publish ports on the VPS; Caddy forwards `https://jpeg.hi5central.com` to `hi5central-jpeg-relay:8787`.

## Preparation

- Verify wildcard DNS `jpeg.hi5central.com` points to the IONOS VPS.
- Verify `docker network inspect hi5central-internal` works.
- Use the existing Caddy wildcard certificate. No additional port 80/443 mappings or new DNS provider credentials needed.

## Deploy relay

```bash
git clone https://github.com/dansut24/Hi5Central-JPEG-WebRTC-.git /opt/hi5-jpeg-webrtc
cd /opt/hi5-jpeg-webrtc
docker build -t hi5-jpeg-relay:local .
docker run -d --name hi5central-jpeg-relay --restart unless-stopped \
  --network hi5central-internal \
  --read-only --tmpfs /tmp:rw,noexec,nosuid,size=16m \
  --cap-drop ALL --security-opt no-new-privileges:true \
  hi5-jpeg-relay:local
```

In the existing `/opt/hi5central/runtime/caddy/Caddyfile`, add this block **inside the existing wildcard site, before the final catch-all `handle`**:

```caddyfile
  # H5C_JPEG_RELAY_BEGIN
  @jpegRelayHost host jpeg.hi5central.com
  handle @jpegRelayHost {
    reverse_proxy hi5central-jpeg-relay:8787
  }
  # H5C_JPEG_RELAY_END
```

Backup the existing Caddyfile before editing, then run:

```bash
docker exec hi5central-gateway-1 caddy validate --config /etc/caddy/Caddyfile
docker exec hi5central-gateway-1 caddy reload --config /etc/caddy/Caddyfile
curl -fsSI https://jpeg.hi5central.com/
```

Test `https://jpeg.hi5central.com/` and make a disposable private session.

## Existing TURN (optional)

Set `STUN_URL`, `TURN_URL`, `TURN_SECRET` on the new relay to point at existing Hi5Central coturn **only after confirming it supports REST/shared-secret temporary TURN credentials** and identifying its public hostname/port. Do not change the TURN container or credentials while testing. Without these variables, browsers will try direct WebRTC using the default STUN server; restrictive networks may fail to connect.

The relay does not proxy JPEG frames itself: Pion and the browser communicate directly when ICE permits. For blocked direct paths, TURN relays the encrypted WebRTC traffic through the VPS. TURN bandwidth is therefore charged to the VPS.

## Stop/rollback

```bash
docker rm -f hi5central-jpeg-relay
```

Remove only the marked JPEG block from Caddyfile, validate and reload. Never replace the whole Caddyfile with this repository's sample.

## Safety

Sessions use random access links, a single viewer, and sender-controlled lifetime. This MVP lacks user-account authentication, public rate limits, signed binaries and compliance review. Never use it for unattended remote access.
