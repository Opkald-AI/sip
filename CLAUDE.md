# CLAUDE.md - sip (Opkald-AI fork of livekit/sip)

Go SIP <-> LiveKit bridge. It's the entry point for **every** inbound clinic phone call: SIP trunk ->
this service -> LiveKit room -> the `livekit` agent.

## State of the fork (2026-10)

- Default branch: `main`. It's a plain mirror of `livekit/sip` (last synced to upstream `d5d1e09`, 2026-07-10;
  ~73 commits behind upstream now). There are **no** OpkaldAI commits on `main`.
- Production (on `livekit-sip-and-backend`, 78.46.187.165 / 10.0.0.4) runs a binary built from
  upstream **`ff5d49a` (2025-06-14)**, not from this fork's `main`. This repo has no deploy workflow;
  the server was set up by hand.
- OpkaldAI work lives on branches by `s171243`, both based on `ff5d49a`:
  - `feature/decline-recent-callers-pinned`: pre-answer decline check that returns **486 Busy Here**.
    Config `decline_check_url` + `decline_check_numbers` (empty list = check never runs). Does a 1.5 s
    fail-open `GET ?from=&to=&trunk=` that expects `{"reject": bool}`. Files: `pkg/config/config.go`,
    `pkg/sip/decline.go`, `pkg/sip/inbound.go`.
  - `feature/decline-recent-callers-486`: earlier iteration of the same idea.
  - Neither has a PR, and it's unknown whether either one is deployed.
- Every other branch is an upstream branch that came along with the fork.

## Commands (upstream tooling)

```bash
mage build                         # needs Go (see go.mod), mage, libopus/libsoxr dev libs
go test ./...                      # unit tests
sip --config=config.yaml           # or SIP_CONFIG_BODY env var
docker compose up                  # local redis + livekit-server --dev + livekit/sip image
```

Not verified in this environment (no Go toolchain).

## Rules / gotchas

- Don't push OpkaldAI changes to `main`. Keep `main` syncable with upstream and build production
  patches on a branch pinned to the commit production actually runs (`ff5d49a`).
- Inherited CI (`.github/workflows/docker.yaml`) pushes to Docker Hub `livekit/sip` with
  `DOCKERHUB_*` secrets that this fork doesn't have. Don't rely on it for images.
- Any change here affects all clinics' inbound calls. Fail open, and gate by called number.
- Config contains LiveKit `api_key`/`api_secret` and trunk creds. Never commit a real `config.yaml`.
