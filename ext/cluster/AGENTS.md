# AGENTS.md — ext/cluster

`Client` (`wrapper.go`) is a dry-run safety wrapper around a real `dclient.Interface`: "Overwrite write actions to dry run — prevents performing actual operations." It holds two clients, `inner` (the real cluster) and `fake` (an empty `dclient.NewEmptyFakeClient()`), and every method has to decide which one it's allowed to touch.

## The contract: writes never reach `inner`, reads do

- **Every resource write method** (`PatchResource`, `DeleteResource`, `CreateResource`, `UpdateResource`, `UpdateStatusResource`, `ApplyResource`, `ApplyStatusResource`) routes unconditionally to `c.fake`. No fallback, no error check, no path to `c.inner` at all. This is what makes the wrapper a real dry run: a write against this client can never mutate the actual cluster, full stop.
- **Reads go the other way.** `ListResource` goes straight to `c.inner` (a dry-run wrapper still needs real data to test policies against). `GetResource` and `RawAbsPath` (GET only) try `c.fake` first — so a resource you wrote through this wrapper earlier is visible again — and fall back to `c.inner` if the fake has nothing.

## `RawAbsPath` is the one method that has to tell reads and writes apart itself

Every other method's Go signature already says whether it's a read or a write. `RawAbsPath` takes an arbitrary HTTP `method` string, so it's both, and has to branch on it explicitly: non-GET routes straight to `c.fake` like the other write methods (no fallback), GET keeps the try-fake-then-fall-back-to-`inner` read pattern. Getting this wrong is exactly how this wrapper broke once already: the fake client hard-errors on every non-GET method (`dclient.rawAbsPathForFakeClient`), so a naive "try fake, fall back to inner on any error" implementation silently sends every raw POST/PUT/DELETE/PATCH straight to the real cluster. If you add another method here that carries its own verb instead of being named for one, branch on the verb the same way `RawAbsPath` does, don't just copy the GET fallback pattern.

## No in-repo caller — this is exported for downstream consumers

`cluster.New` (the real, non-fake constructor in `cluster.go`) has no caller inside kyverno/kyverno itself; `grep -rn "cluster.New("` turns up nothing. This package is built for use outside this repo (it imports `github.com/kyverno/playground/backend/data`), so a regression here doesn't show up in kyverno's own test suite failing in some obviously-connected way, it shows up as a downstream consumer's dry-run guarantee silently not holding. Test this package's own safety contract directly (see `wrapper_test.go`'s `spyClient` pattern), don't rely on an internal caller to exercise it.
