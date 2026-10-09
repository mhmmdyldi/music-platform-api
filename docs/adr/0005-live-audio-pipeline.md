# 0005. MediaMTX ingest and HLS delivery for live audio

**Status:** accepted

## Context

Listeners hear a live set but do not interact with the DJ in real time. The
audience per room can be large and global. Real DJs and the AI DJ must
publish the same way: the core platform is source-agnostic.

## Decision

- **Media server: MediaMTX.** It is open source, a single binary or container,
  and accepts RTMP and SRT (and WebRTC/WHIP if enabled). It outputs Low-Latency HLS.
- **Delivery: LL-HLS through a CDN** (CloudFront in AWS). Expected latency is a
  few seconds, and the cost is dominated by cheap CDN egress.
- **Control stays in the backend:** MediaMTX will call the backend to
  authorise each publish (HTTP auth hook) and report stream start and stop. The
  backend never carries audio.
- Every Music Source (human DJ software, the AI DJ service, future sources)
  publishes through the same ingest path with a per-source credential.
- The local stack runs MediaMTX plus an ffmpeg test source, so the pipeline
  is testable on a laptop.

## Consequences

- No sub-second interaction. If interactive rooms are ever needed, a WebRTC
  engine (for example LiveKit) can serve those rooms behind the same Music
  Source model and Source API.
- Horizontal scaling of MediaMTX (origin/edge layout, failover) is a later
  infrastructure decision.
- LL-HLS parts must align with audio frames for iOS players (see
  `deploy/local/mediamtx.yml`); production sources will need a known audio
  format (for example AAC, 48 kHz).
