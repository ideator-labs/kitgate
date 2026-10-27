# AGENTS.md: KitGate handoff

## What this is and the claim

A Go CLI that fails CI when a Docker Sandbox Kit upgrade widens the SPEC-v3 §7.4 permission surface. Claim: when Renovate or a teammate bumps a Kit, KitGate reads the old and new descriptors (YAML, published JSON, or the `vnd.docker.sandbox.kit.descriptor` OCI annotation), projects each with the official `github.com/docker/sandbox-kit-spec/v3/spec` library (`SurfaceOf` + `DiffWidenings`), and exits 1 on any widening, including a **removed deny** rule.

## Status

Real:
- Projection and widen detection come from `sandbox-kit-spec` `v3.0.0-m.11` (`spec.SurfaceOf`, `spec.DiffWidenings`). KitGate does not reimplement §7.4.
- Fixtures start from the real `examples/gh/gh.yaml` in that module (GitHub CLI mixin with a `DELETE /repos/**` deny).
- OCI path reads a local `manifest.json` annotation. No registry pull and no Docker daemon.

Stubbed / not built:
- No Renovate or GitHub Action wrapper yet.
- No live `docker pull` of a published Kit. Point it at descriptors or a local OCI layout.
- No set-merge / multi-kit composition gate (one Kit at a time).
- Firecracker / QEMU / Docker not used. This POC needs only Go.

## Data shape

`spec.Surface` is the permission surface: phased network allow/deny (host and HTTP method/path projections), credentials by phase, SSH agent, storage, host mounts, skills paths, ports, USB, privileged, and `type+config-digest` for other requests. Types such as `lifecycle@1` and `agent-context@*` contribute nothing.

`[]spec.Widening` is the gate result. Each entry has `Category` (for example `network.runtime.http.deny-removed`) and `Detail` (for example `DELETE api.github.com/repos/**`).

## Repo map

- `main.go`: `surface`, `lock`, `check`, `diff`. Exit 0 ok, 1 widening, 2 bad input. `--ignore-deny-removed` is the deliberate break switch.
- `load.go`: YAML/JSON descriptor, OCI image manifest annotation, or a directory with `manifest.json` / `index.json`.
- `fixtures/`: derived from `examples/gh/gh.yaml` at module `v3.0.0-m.11`.
- `demo.sh`: the artifact. 9 checks plus `--break`.

## Fresh clone: setup, run, verify

```
git clone https://github.com/ideator-labs/kitgate && cd kitgate
go test ./...                 # no package tests yet; build is the check
bash demo.sh                  # expect "RESULT: 9 passed, 0 failed", exit 0
bash demo.sh --break          # expect "RESULT: 6 passed, 3 failed", exit 1
```

Needs Go 1.26.10+ (module floor from sandbox-kit-spec). Tested with Go 1.27.3 on x86_64 Linux. No Docker, QEMU, network, or paid API.

## Design decisions

- Call the normative library instead of copying §7.4. A SPEC change that lands in the module is the source of truth.
- Accept authored YAML and the published annotation form so CI can gate a Renovate bump of an image digest without rebuilding Kits locally.
- Fail closed on missing annotation or bad input (exit 2), never a silent pass.
- `--ignore-deny-removed` exists only so `demo.sh --break` proves the deny rule is load-bearing.

## Limits

- One Kit at a time. A `kind: set` merge is not composed here.
- Create-phase arg expansion is not applied. Fixtures have no create-phase args that change policy.
- OCI support is local files only. No registry fetch.
- Fixtures for widen cases are hand-edited from the real gh example. They are not published images.
- Crowding claim: `kit-tck` has `validate` and `inspect` but no surface diff (verified in `cmd/kit-tck` of the same module). The sbx runtime gates upgrades on the laptop. KitGate is the PR-time diff.

## Next steps (prioritized, each with done-when)

1. GitHub Action. Done when a sample workflow fails a PR that drops the DELETE deny and posts the widenings in the job summary.
2. Registry input: `kitgate diff old-ref new-ref` via ORAS. Done when it diffs two digests from a local registry without Docker.
3. Effective-descriptor args: expand create-phase args before `SurfaceOf`. Done when a fixture whose allow list is `${{ kit.args.host }}` gates on the resolved value.
4. Multi-kit / set: project the merged surface of a set. Done when a set that adds a host fails the same way a single Kit does.

## Working rules

Commit as danielgerlag <daniel@gerlag.ca>. Keep `bash demo.sh` green and `--break` red. No em-dashes. No invented links. Label stubs.

## Sources

- Kit Spec v3 (Sep 24, 2026): https://www.docker.com/blog/docker-sandbox-kit-spec/
- Normative §7.4 and capability pages: https://raw.githubusercontent.com/docker/sandbox-kit-spec/main/docs/spec/SPEC-v3.md
- Go module used here: `github.com/docker/sandbox-kit-spec/v3@v3.0.0-m.11` (`spec/surface.go`)
- Docker Cloud Sandboxes (Oct 1, 2026): https://www.docker.com/blog/docker-cloud-sandboxes-wearedevelopers-recap/
- Sandboxes 0.48.0 notes (Oct 20-21, 2026): https://collabnix.com/docker/2026/10/
