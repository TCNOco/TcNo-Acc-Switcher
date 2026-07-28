# Steam Guard protocol HTTP boundary

This package owns outbound HTTP policy and the bounded authentication-session
operations used by Steam Guard. It does not implement password RSA encryption,
authenticator enrollment, web cookies, confirmations, or automatic retries.

Initial requests use one of two routes:

- `RouteRequest`: `api.steampowered.com`, `login.steampowered.com`, or
  `steamcommunity.com`.
- `RouteTransfer`: `steamcommunity.com`, `store.steampowered.com`, or
  `help.steampowered.com`.

Every endpoint and redirect must use HTTPS on the default TLS port. Redirects
are disabled unless a request opts in. An allowed redirect receives only the
fixed user agent and `Accept-Encoding: identity`; cookies, authorization,
referrers, and other caller headers are discarded.

Each call supplies a timeout. Request bodies stop at 1 MiB and response bodies
stop at 4 MiB. A request may choose a smaller response limit. The client does
not return raw response headers, error bodies, URLs, or underlying transport
errors. It converts `Retry-After` and a canonical numeric `X-EResult` into typed
metadata. A malformed `X-EResult` fails closed.

## Integration contract

Create one client for a Steam Guard session and close its idle connections when
the session locks:

```go
client := protocol.NewClient(protocol.Options{})
defer client.CloseIdleConnections()

result, err := client.Do(ctx, protocol.Request{
	Method:   http.MethodPost,
	Endpoint: "https://api.steampowered.com/...",
	Route:    protocol.RouteRequest,
	Header:   http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
	Body:     payload,
	Timeout:  15 * time.Second,
})
```

Inspect errors with `errors.As(err, &protocolError)`. `Code` identifies the
failure and `State` tells the service whether retry is permitted. The package
does not retry POST requests. A higher layer must apply `RetryAfter`, user
cancellation, and operation-specific replay rules.

Successful response bodies can contain tokens. Callers must not log them and
must release or overwrite parsed secret buffers when the operation finishes.

## Authentication session contract

`AuthenticationClient` exposes these Steam Web API operations:

- `BeginAuthSessionViaCredentials`
- `UpdateAuthSessionWithSteamGuardCode`
- `PollAuthSessionStatus`
- `GenerateAccessTokenForApp`
- `GetAuthSessionInfo`
- `UpdateAuthSessionWithMobileConfirmation`

The wire fields match Steam's
[`steammessages_auth.steamclient.proto`](https://github.com/SteamTracking/Protobufs/blob/6d9868869cd3f3f7326dcd7f0fecff32d4a0a4d3/steam/steammessages_auth.steamclient.proto).
The package writes and reads only the fields required by these calls. Unknown
fields, duplicate singleton fields, non-canonical varints, unsupported wire
types, invalid enum values, and oversized values return `CodeInvalidResponse`.

The reviewed local reference is
[`SteamMsgAuth.cs`](https://github.com/SteamRE/SteamKit/blob/ccdffb73866445deddb50f0a04117e60d585a368/SteamKit2/SteamKit2/Base/Generated/SteamMsgAuth.cs)
at SteamKit commit `ccdffb73866445deddb50f0a04117e60d585a368`. Its
generated-file header names `steammessages_auth.steamclient.proto` as the input,
and the tracked Git blob's SHA-256 is
`4d863642c8c874d3c315da66629ce7f1b585142682234185c8024051f7475090`.
The Windows checkout applies the repository's `eol=crlf` attribute and hashes
to `ddab37c45a8cacd2177cb294feef2ffeaaad95bbbe97dd140a4f9aaa459f3051`.
That SteamKit commit pins `Resources/Protobufs` to SteamTracking/Protobufs
commit `6d9868869cd3f3f7326dcd7f0fecff32d4a0a4d3`.

The raw protobuf submodule was not initialized in the review checkout. The
commit link above is immutable, but the raw `.proto` SHA-256 remains unverified.
Do not treat the raw-schema hash gate as complete until those exact commit bytes
are available locally and hashed.

```go
auth := protocol.NewAuthenticationClient(client)

begin, err := auth.BeginAuthSessionViaCredentials(ctx, input, 15*time.Second)
if err != nil {
	return err
}
defer begin.Session.Destroy()

poll, err := auth.PollAuthSessionStatus(ctx, begin.Session, 15*time.Second)
```

Credential begin creates a random 128-bit local session ID with `crypto/rand`.
The ID is not sent to Steam. `AuthSession` keeps Steam's request ID private and
uses it only for polling. Call `Destroy` when polling completes or the owning
vault locks.

`AuthResultState` tells the caller to wait, request a challenge, open an
agreement URL, or store issued tokens. Access tokens, refresh tokens, guard
data, associated email hints, encrypted passwords, mobile signatures, and raw
protobuf bodies must not be logged. The client wipes mutable request, form, and
response buffers before returning where the Go type permits it. Returned token
strings must move directly into protected storage.

QR approval uses a MobileApp access token in the URL-escaped `access_token`
query parameter. The token is never included in a typed error. Call
`GetAuthSessionInfo` first and show its bounded requestor metadata for explicit
approval. `UpdateAuthSessionWithMobileConfirmation` requires a canonical
`X-EResult: 1` and an empty protobuf response; a missing, malformed, or
non-success result cannot be reported as accepted. The QR package remains the
sole owner of challenge parsing and signature construction.
