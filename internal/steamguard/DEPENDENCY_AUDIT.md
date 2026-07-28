# Steam Guard dependency audit

Audit date: 2026-07-22. Shipped scope: the Go application and embedded frontend. The ignored SteamAuth and SteamKit reference checkouts are review inputs, not runtime components.

## Production decision

Steam Guard uses the local `internal/steamguard/protocol` HTTP and protobuf implementation. It does not import SteamKit, SteamAuth, `g-man`, a protobuf runtime, cgo, a helper executable, or a bundled native DLL.

The selected module graph contains no `g-man`, `aoni`, `miyako`, uTLS, or QUIC module. Their cookie, redirect, debug-dump, and transfer-URL risks do not enter this build.

The feature added one production Go module:

| Component | Pin and source | Release | License | Runtime closure |
| --- | --- | --- | --- | --- |
| `github.com/piglig/go-qr` | `v1.1.0`, commit `832517b8dd4c5f48188211c0c6691c6ac38b0363`, module sum `h1:awmlkMeSKfrXDm7OlxrOM6LMgL9LOYjPRSdCCYemNyA=` | 2026-05-29 | MIT, local `LICENSE` SHA-256 `e1149e499b7274df994c1653c6edcd271c775249f6f6db93b636ad06e7e109c2` | Standard library plus its internal Reed-Solomon package |

The [tagged repository](https://github.com/piglig/go-qr/tree/v1.1.0) is published under the `piglig` account. The module artifact does not establish a separate maintainer roster, so ownership remains a single-publisher supply-chain risk. Adoption on 2026-07-22 occurred 54 days after publication, beyond the repository's 24-hour cooling period.

The selected package is pure Go. Its compiled package contains no cgo files, `unsafe`, network client, subprocess, runtime plugin, request logging, generated-code marker, or generation directive. It does expose unrelated file rendering helpers from the same package, but this application calls only the in-memory `Decode` function. Upstream `testify`, `spew`, `difflib`, and `yaml.v3` requirements are test-only and do not appear in the application's runtime package closure.

The wrapper caps an encoded screenshot at 8 MiB, dimensions at 8192 by 8192, decoded pixels at 24 million, candidate frames at 8, decode regions at 5 per frame, retained payloads at 2, and checked elapsed time at 3 seconds. Panics are converted to typed decoder failures. The upstream module contains two decoder fuzz targets; `internal/steamguard/qrimage` contains five wrapper fuzz targets.

## Existing modules used by Steam Guard

Steam Guard also uses modules already present in the application: `github.com/google/uuid v1.6.0`, `github.com/wailsapp/wails/v3 v3.0.0-alpha2.117`, `golang.org/x/crypto v0.53.0`, `golang.org/x/image v0.43.0`, `golang.org/x/net v0.56.0`, and `golang.org/x/sys v0.46.0`. Minimal Version Selection resolves `golang.org/x/text v0.39.0`.

The Steam Guard package closure has no cgo packages. Local Windows implementations use `unsafe` only at reviewed Win32 boundaries and call operating-system DLLs through `x/sys/windows`. No third-party native library is shipped.

The protocol transport disables proxy environment variables, requires HTTPS and the default TLS port, keeps certificate verification enabled, enforces TLS 1.2 or later, strips caller headers on redirects, caps bodies and headers, and applies operation deadlines. Its request policy names only these hosts:

- request hosts: `api.steampowered.com`, `login.steampowered.com`, `steamcommunity.com`
- transfer hosts: `steamcommunity.com`, `store.steampowered.com`, `help.steampowered.com`

No production Steam Guard request currently opts into redirects. The package contains no logging calls. Typed errors omit URLs, bodies, headers, cookies, tokens, and underlying transport errors.

## Protobuf provenance

The protocol uses a handwritten, bounded wire codec. No `google.golang.org/protobuf` or `github.com/golang/protobuf` package appears in its package closure, and release builds perform no code generation.

`internal/steamguard/protocol/README.md` pins the authentication schema to SteamTracking/Protobufs commit `6d9868869cd3f3f7326dcd7f0fecff32d4a0a4d3`. That commit is the `Resources/Protobufs` gitlink in the reviewed SteamKit commit `ccdffb73866445deddb50f0a04117e60d585a368`. The tracked generated `SteamMsgAuth.cs` identifies `steammessages_auth.steamclient.proto` as its input and has Git-blob SHA-256 `4d863642c8c874d3c315da66629ce7f1b585142682234185c8024051f7475090`.

The raw protobuf submodule was not initialized in the local review checkout. Its commit URL is immutable, but its raw `.proto` SHA-256 remains unverified. Keep the raw-schema hash gate open until those exact commit bytes are available locally and hashed.

## Verification and advisories

The following offline commands passed with Go 1.26.4:

```text
go mod verify
go list -m -json all
go mod graph
go test -count=1 ./internal/steamguard/qrimage ./internal/steamguard/protocol ./internal/steamguard/confirmationapi ./internal/steamguard/enrollmentapi
go vet ./internal/steamguard/qrimage ./internal/steamguard/protocol ./internal/steamguard/confirmationapi ./internal/steamguard/enrollmentapi
```

The generated `OPEN_SOURCE_LICENSES.txt` inventories 380 components with no missing license text. It contains the exact MIT text from `github.com/piglig/go-qr v1.1.0`; no notice regeneration is required for the current manifests.

`govulncheck` is not installed in the controlled workspace and was not downloaded during this audit. The `govulncheck ./...` gate remains open. A package-graph check found none of the currently relevant advisory packages: `golang.org/x/crypto/openpgp`, `golang.org/x/image/tiff`, `golang.org/x/net/idna`, or `golang.org/x/net/http2`. The application uses `x/image/webp` at `v0.43.0`, the fixed version for [GO-2026-5061](https://pkg.go.dev/vuln/GO-2026-5061), and `x/net/html` at `v0.56.0`, newer than the fixed version for [GO-2026-5030](https://pkg.go.dev/vuln/GO-2026-5030). This manual reachability check does not replace `govulncheck`.

The only `replace` directive selects `github.com/Jleagle/unmarshal-go` at the exact pseudo-version `v0.0.0-20260702203424-38325863b365`. It predates this feature and is not in the current package build graph. Its review status is outside this Steam Guard dependency change, but the repository-wide no-unreviewed-replacement gate should not be checked without separate provenance.
