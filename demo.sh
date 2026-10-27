#!/usr/bin/env bash
# KitGate demo: a Kit upgrade that drops a deny rule (or adds a host / privileged)
# fails CI; a provides-version bump alone passes. Needs Go 1.24+.
# Usage: bash demo.sh          (all checks pass, exit 0)
#        bash demo.sh --break  (ignores deny-removed; check 3 fails, exit 1)
set -uo pipefail
cd "$(dirname "$0")"
OUT=.demo
rm -rf "$OUT"
mkdir -p "$OUT"

EXTRA=()
if [[ "${1:-}" == "--break" ]]; then
  echo "BREAK MODE: --ignore-deny-removed (SPEC-v3 7.4 says a removed deny is a widening)"
  EXTRA=(--ignore-deny-removed)
fi

go build -o "$OUT/kitgate" . || exit 2
KG="$OUT/kitgate"

PASS=0
FAIL=0
check() {
  if [[ "$2" == ok ]]; then
    echo "PASS $1"
    PASS=$((PASS + 1))
  else
    echo "FAIL $1"
    FAIL=$((FAIL + 1))
  fi
}

# 1. Unchanged kit vs itself: 0 widenings.
"$KG" "${EXTRA[@]}" diff fixtures/gh-v1.yaml fixtures/gh-v1.yaml >"$OUT/1.txt" 2>"$OUT/1.err"
rc=$?
[[ $rc == 0 ]] && grep -q '^0 widenings$' "$OUT/1.txt" && r=ok || r=no
check "1 same kit vs itself is clean (exit $rc)" "$r"

# 2. provides version bump alone is not a widening.
"$KG" "${EXTRA[@]}" diff fixtures/gh-v1.yaml fixtures/gh-patch.yaml >"$OUT/2.txt" 2>"$OUT/2.err"
rc=$?
[[ $rc == 0 ]] && grep -q '^0 widenings$' "$OUT/2.txt" && r=ok || r=no
check "2 provides bump gh@2.103.0 to gh@2.104.0 passes (exit $rc)" "$r"

# 3. Dropping DELETE /repos/** deny must fail (SPEC-v3 7.4).
"$KG" "${EXTRA[@]}" diff fixtures/gh-v1.yaml fixtures/gh-widen-deny.yaml >"$OUT/3.txt" 2>"$OUT/3.err"
rc=$?
if [[ $rc == 1 ]] && grep -q 'network.runtime.http.deny-removed: DELETE api.github.com/repos/\*\*' "$OUT/3.txt"; then
  r=ok
else
  r=no
fi
check "3 removed deny is blocked (exit $rc; $(grep -c 'deny-removed' "$OUT/3.txt" || true) deny-removed lines)" "$r"
grep 'deny-removed\|widening' "$OUT/3.txt" | sed 's/^/     /' || true

# 4. New allow host fails.
"$KG" "${EXTRA[@]}" diff fixtures/gh-v1.yaml fixtures/gh-widen-host.yaml >"$OUT/4.txt" 2>"$OUT/4.err"
rc=$?
[[ $rc == 1 ]] && grep -q 'network.runtime.allow: evil.example.com' "$OUT/4.txt" && r=ok || r=no
check "4 new host evil.example.com is blocked (exit $rc)" "$r"

# 5. Privileged request fails.
"$KG" "${EXTRA[@]}" diff fixtures/gh-v1.yaml fixtures/gh-widen-priv.yaml >"$OUT/5.txt" 2>"$OUT/5.err"
rc=$?
[[ $rc == 1 ]] && grep -q 'privileged: requests privileged execution' "$OUT/5.txt" && r=ok || r=no
check "5 privileged@1 is blocked (exit $rc)" "$r"

# 6. agent-context text change is not permission surface.
"$KG" "${EXTRA[@]}" diff fixtures/gh-v1.yaml fixtures/gh-context-only.yaml >"$OUT/6.txt" 2>"$OUT/6.err"
rc=$?
[[ $rc == 0 ]] && grep -q '^0 widenings$' "$OUT/6.txt" && r=ok || r=no
check "6 agent-context-only edit passes (exit $rc)" "$r"

# 7. Surface of the real gh example has the deny and the github credential.
"$KG" surface fixtures/gh-v1.yaml >"$OUT/7.json" 2>"$OUT/7.err"
rc=$?
python3 - "$OUT/7.json" <<'PY' && r=ok || r=no
import json,sys
s=json.load(open(sys.argv[1]))
assert "github.com" in s.get("networkRuntimeAllow",[])
assert "DELETE api.github.com/repos/**" in s.get("networkRuntimeHTTPDeny",[])
assert "github" in s.get("credentialsRuntime",[])
assert s.get("privileged") in (None, False)
print("surface keys ok")
PY
[[ $rc == 0 ]] || r=no
check "7 surface of fixtures/gh-v1.yaml matches SPEC projection" "$r"

# 8. Same deny-removed catch when descriptors come from OCI manifest annotations.
"$KG" "${EXTRA[@]}" diff fixtures/oci-v1 fixtures/oci-widen-deny >"$OUT/8.txt" 2>"$OUT/8.err"
rc=$?
if [[ $rc == 1 ]] && grep -q 'network.runtime.http.deny-removed: DELETE api.github.com/repos/\*\*' "$OUT/8.txt" \
  && grep -q 'oci-annotation' "$OUT/8.err"; then
  r=ok
else
  r=no
fi
check "8 OCI annotation path catches the same deny removal (exit $rc)" "$r"

# 9. lock + check: lock v1, check patch passes, check widen-deny fails.
"$KG" lock fixtures/gh-v1.yaml "$OUT/surface.lock" >"$OUT/9a.txt"
"$KG" "${EXTRA[@]}" check fixtures/gh-patch.yaml "$OUT/surface.lock" >"$OUT/9b.txt" 2>"$OUT/9b.err"
rc_patch=$?
"$KG" "${EXTRA[@]}" check fixtures/gh-widen-deny.yaml "$OUT/surface.lock" >"$OUT/9c.txt" 2>"$OUT/9c.err"
rc_widen=$?
if [[ $rc_patch == 0 && $rc_widen == 1 ]] && grep -q 'deny-removed' "$OUT/9c.txt"; then
  r=ok
else
  # In --break mode, widen-deny check returns 0, so this fails as intended.
  r=no
fi
check "9 lock/check: patch passes, deny-removed fails (patch=$rc_patch widen=$rc_widen)" "$r"

echo "RESULT: $PASS passed, $FAIL failed"
if [[ $FAIL == 0 ]]; then
  echo "RESULT: PASS"
  exit 0
fi
echo "RESULT: FAIL"
exit 1
