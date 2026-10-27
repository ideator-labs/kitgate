# KitGate

CI gate for Docker Sandbox Kit upgrades. When a Kit asks for more authority than the version you already approved, the pull request fails.

A Kit is an ordinary OCI image. Its manifest annotation `vnd.docker.sandbox.kit.descriptor` declares what the host must grant: network allow and deny lists, credentials, mounts, ports, privileged, and the rest of SPEC-v3 §7.4. KitGate projects the old and new descriptors onto that permission surface with the official Go library and exits 1 on any widening. Removing a deny rule counts as a widening.

```
kitgate diff  old.yaml new.yaml     # exit 0 ok, 1 widening, 2 bad input
kitgate lock  kit.yaml surface.lock
kitgate check kit.yaml surface.lock
kitgate surface kit.yaml            # print the surface as JSON
```

Inputs may be authored YAML, published descriptor JSON, an OCI image `manifest.json` that carries the annotation, or a directory containing that manifest.

## Demo

```
bash demo.sh          # 9 checks, exit 0
bash demo.sh --break  # ignores deny-removed; 3 checks fail, exit 1
```

Needs Go 1.26.10+ (module floor from sandbox-kit-spec). No Docker daemon, no registry, no paid API.

The fixtures start from the real `examples/gh/gh.yaml` in `github.com/docker/sandbox-kit-spec/v3@v3.0.0-m.11`. That Kit allows GitHub API methods including DELETE, then denies `DELETE /repos/**`. A patch that only bumps `provides` from `gh@2.103.0` to `gh@2.104.0` passes. Dropping the deny fails with:

```
1 widening(s):
  + network.runtime.http.deny-removed: DELETE api.github.com/repos/**
```

Adding `evil.example.com` or `privileged@1` also fails. An `agent-context` text edit does not. The same deny removal is caught when both sides are OCI manifests that carry the descriptor annotation.

Setup, design, and limits are in [AGENTS.md](AGENTS.md).

## Honest limits

One Kit at a time. Create-phase arg expansion is not applied. OCI input is local files only. This is a proof of concept, not a released tool.

MIT licensed.
