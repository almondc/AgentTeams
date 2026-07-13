# WOODFIELD-PATCHES.md

## Overview

This is a **temporary** patched fork of upstream HiClaw (AgentTeams) `v1.1.2`, maintained for a
self-hosted deployment running the controller in Kubernetes `incluster` mode against self-hosted
MinIO and a self-hosted Higress gateway/console. Every patch on this branch fixes a defect that was
**live-reproduced** against a real cluster and **source-verified** against the actual
`agentscope-ai/AgentTeams` code — none of these are speculative fixes. The goal is for this fork to
be short-lived: once a released upstream version contains all of these fixes, this fork should be
dropped and the deployment repo pointed back at stock upstream images.

## Base

- Upstream repository: `agentscope-ai/AgentTeams`
- Base tag: **`v1.1.2`**, commit `a99457830fafb99c991bdb666aa8a1eef2f83b12`
- Confirmed exact: `git merge-base` of this branch's HEAD against `fork/main` resolves to
  `a994578`, and `git ls-remote --tags fork`/`git ls-remote --tags origin` both resolve
  `refs/tags/v1.1.2` to the same commit — i.e. this branch is 13 commits cherry-picked directly
  onto the upstream `v1.1.2` tag, not onto a moving `main`.
- Branch: `woodfield/v1.1.2-patched` (this branch), 13 patch commits on top of `a994578`.
- Tag on this fork: `v1.1.2-woodfield` (annotated, at the HEAD that includes this file).

## The eleven fixes

Eleven confirmed upstream bugs map to **13 commits** on this branch: fix #7 required 4 separate
commits (iterated on live), and fix #8 required **no fork commit at all** — it's a DNS/routing
problem solved entirely in the deployment repo with a Kubernetes `hostAliases` overlay
(`k8s/hiclaw/agent-pod-template-configmap.yaml` in the ops repo), not a code change here. That's
why 11 bugs and 13 commits both reconcile: 9 bugs × 1 commit, 1 bug (#7) × 4 commits, 1 bug (#8) ×
0 commits = 13.

| # | Symptom | Root cause | Commit(s) on this branch | Image(s) affected |
|---|---------|------------|---------------------------|--------------------|
| 1 | CoPaw worker crash-loops against self-hosted MinIO in k8s mode | `copaw_worker/sync.py`'s `FileSync._ensure_alias()` unconditionally skips `mc alias set` whenever `HICLAW_RUNTIME=k8s`, assuming a credential-injection sidecar always sets `MC_HOST_<alias>` — not true for self-hosted MinIO | `725a665` | copaw-worker |
| 2 | Team workers push results to the wrong MinIO path; Leader silently redoes the work and reports success | `hiclaw-controller`'s `buildCoordinationBlock()` (`internal/agentconfig/coordination.go`) only emits a `**Team**:` line for the Team Leader's AGENTS.md, never for plain workers; hermes-worker's `FileSync._get_team_id()` and `push-shared.sh` key off that exact marker, so every team worker looks standalone and syncs to the global bucket prefix instead of `teams/<team>/shared/` | `0a9fb4c` | controller |
| 3 | Team Leader has no way to address its own 1:1 room with Manager, so it can't report completion upward | `CoordinationContext`/`CoordinationDeployRequest` never carried the leader's own room ID; the call site in `reconcileTeamNormal` (Step 3.5) never looked it up from `Status.Members` before calling `InjectCoordinationContext` | `4da8bbe` | controller |
| 4 | `spec.skills` (on-demand skills) never install in incluster mode — first failure mode: "No such file or directory" / "log: command not found" | `hiclaw-controller/Dockerfile` never copied `shared/lib/` or `manager/scripts/lib/` into the image, even though the controller itself directly executes `agent/skills/worker-management/scripts/push-worker-skills.sh` (which sources them) in incluster mode | `9bb827b` | controller |
| 5 | `spec.skills` — second half: `ERROR: Worker '<name>' not found in registry` even after fix 4 | `push-worker-skills.sh`'s only source of truth is a local `${HOME}/workers-registry.json` that the embedded/Docker Manager maintains continuously; nothing in the incluster controller ever wrote it | `0f7a6a8` | controller |
| 6 | Manager Pod has no working Higress Console credential in k8s mode; any skill that logs in to the Console (`mcp-server-management`, `git-delegation-management`) fails with no session cookie | `manager/scripts/init/start-manager-agent.sh`'s `HICLAW_RUNTIME=k8s` branch assumes `ManagerReconciler` injects `HICLAW_ADMIN_PASSWORD`/console URL — but `ManagerReconciler.createManagerContainer` never actually did | `2eb9bc1` | controller (adds `CreateRequest.SecretEnv`, wires it only into `ManagerReconciler.createManagerContainer`; the Manager Pod's env is populated by the controller, but the fixed code lives in the controller binary/image) |
| 7 | `mcp-server-management` scripts fail in k8s mode even after fix 6 lets the Manager log in — every Console API call gets HTTP 000 | `setup-mcp-server.sh`/`setup-mcp-proxy.sh` (`manager/agent/skills/mcp-server-management/scripts/`) hardcode `CONSOLE_URL="http://127.0.0.1:8001"` and hardcode every `mcporter.json` entry's gateway port, instead of respecting the env vars `gateway-api.sh` already uses | `865d547`, `674e406`, `bc8e7d7`, `0e75c60` | manager (direct `COPY manager/agent/` in `manager/Dockerfile`) **and** controller (same script tree staged into the controller image via the Makefile's `agent/` copy, since the controller also executes these scripts directly in incluster mode) |
| 8 | `aigw-local.hiclaw.io` resolves to the public `127.0.0.1` DNS answer inside the cluster, breaking every script/config that references it | `*-local.hiclaw.io` subdomains are intentionally public-DNS'd to `127.0.0.1` for embedded/Docker-mode convenience; nothing overrides that in k8s mode | **none — not a fork commit.** Fixed with a `hostAliases` entry in the *deployment* repo (`k8s/hiclaw/agent-pod-template-configmap.yaml`), mapping the hostname to the `higress-gateway` ClusterIP for every agent Pod. Called out here so the "11 bugs / 13 commits" count reconciles. | N/A (deployment-repo workaround only) |
| 9 | `spec.mcpServers` (declarative MCP tools for a worker) silently never reaches any worker in incluster mode | The controller's `DeployWorkerConfig` wrote the generated config to the legacy `agents/<name>/mcporter-servers.json`; the `mcporter` 0.11.3 shipped in current worker images only reads `config/mcporter.json` | `1373023` | controller |
| 10 | hermes-runtime team workers silently idle and never receive delegated-task notifications in their team room | The hermes worker never accepted pending Matrix room invites at startup (CoPaw already had this via its `notify_matrix` stage); the hermes gateway's matrix platform only subscribes to rooms it's already joined to at initial sync | `2da76ae` | hermes-worker (first patch to this image — previously stock) |
| 11 | The Kubernetes MCP server 401s (`the server has asked for the client to provide credentials`) even with correct ServiceAccount/RBAC | `GenerateMcporterConfig` unconditionally injects `Authorization: Bearer <consumer-key>` on every `mcpServers` entry; `containers/kubernetes-mcp-server` does credential passthrough — it treats *any* incoming Bearer header as its own kube token instead of its in-cluster SA token | `30d02ff` | controller (new `MCPServer.NoGatewayAuth` field, opted into per-server) |

Full narrative detail (reproduction steps, verification, and the deeper issues each fix exposed) is
in the deployment repo's `docs/ai-lab-architecture.md`, section "Temporary patched HiClaw fork".

## Built images

These tags are built from this branch and referenced in the deployment repo's
`k8s/hiclaw/helmrelease.yaml` (confirmed against that file's `controller.image`, `manager.image`,
and `worker.defaultImage.{copaw,hermes}` values):

- `ghcr.io/almondc/hiclaw-controller:v1.1.2-woodfield8`
- `ghcr.io/almondc/hiclaw-manager:v1.1.2-woodfield4`
- `ghcr.io/almondc/hiclaw-hermes-worker:v1.1.2-woodfield1`
- `ghcr.io/almondc/hiclaw-copaw-worker:v1.1.2-woodfield1`

All four are private GHCR packages, pulled via an `imagePullSecret` (`ghcr-pull`) in the
deployment repo — not public. The differing version suffixes reflect how many times each image
needed a rebuild during the fix-and-verify cycle (the controller absorbed the most fixes: 8, 9, 6,
3, 4, 5, 11, 2, plus the fix-7 script staging it shares with the manager image).

## Build instructions

Builds are driven by the root `Makefile`. The controller build is a prerequisite for the manager
build (the Manager Dockerfile pulls the `hiclaw` CLI binary out of the built controller image), so
build/push the controller first.

**Controller build** (target: `build-hiclaw-controller` / `push-hiclaw-controller`). Before
invoking Docker, the Makefile stages two directories into `./hiclaw-controller/` so the image can
see them despite its build context being scoped to `./hiclaw-controller/`:

```makefile
@rm -rf ./hiclaw-controller/agent && cp -r ./manager/agent ./hiclaw-controller/agent
@rm -rf ./hiclaw-controller/scripts-lib && cp -r ./shared/lib ./hiclaw-controller/scripts-lib && cp -r ./manager/scripts/lib/. ./hiclaw-controller/scripts-lib/
```

`hiclaw-controller/Dockerfile` then does `COPY agent/ /opt/hiclaw/agent/` and
`COPY scripts-lib/ /opt/hiclaw/scripts/lib/` — this staging step is exactly what fix 4 (above)
added; both directories are removed again after the build.

To reproduce the exact published tags (multi-arch, pushed straight to GHCR — set `REGISTRY`/`REPO`
so the Makefile's `$(REGISTRY)/$(REPO)/<image>` pattern resolves to `ghcr.io/almondc/<image>`):

```bash
# 1. Controller — must be built/pushed first; manager's Dockerfile pulls the `hiclaw` CLI from it
make push-hiclaw-controller REGISTRY=ghcr.io REPO=almondc VERSION=v1.1.2-woodfield8

# 2. Manager (depends on the controller image above already being pushed at that tag)
make push-manager REGISTRY=ghcr.io REPO=almondc VERSION=v1.1.2-woodfield4

# 3. CoPaw worker
make push-copaw-worker REGISTRY=ghcr.io REPO=almondc VERSION=v1.1.2-woodfield1

# 4. Hermes worker
make push-hermes-worker REGISTRY=ghcr.io REPO=almondc VERSION=v1.1.2-woodfield1
```

Notes:
- These `push-*` targets build multi-arch (`linux/amd64,linux/arm64`) via `docker buildx` and push
  directly — there's no separate local `build` + `docker push` step for a multi-arch release.
- Each image was rebuilt independently as fixes landed, which is why the four tags carry different
  `-woodfieldN` suffixes rather than one shared version.
- `make push-manager` and the worker `push-*` targets reference `$(CONTROLLER_TAG)` as
  `HICLAW_CONTROLLER_IMAGE` — that tag must already exist in the registry (pushed in step 1) before
  building the images that depend on it, since buildx resolves `FROM ${HICLAW_CONTROLLER_IMAGE}`
  against the registry, not a local Docker image cache, for multi-arch builds.
- For a single native-arch/local dev build instead, use `make build-hiclaw-controller`,
  `make build-manager`, `make build-copaw-worker`, `make build-hermes-worker` (no registry push).

## Status

Upstream PRs are drafted (one clean, single-fix branch per bug, listed in
`docs/ai-lab-architecture.md` in the deployment repo) but **not yet opened** against
`agentscope-ai/AgentTeams`. This fork exists only to unblock this deployment in the meantime — the
intent is to delete it and revert the deployment repo's `helmrelease.yaml` back to stock upstream
images as soon as a released upstream version contains all eleven fixes.
