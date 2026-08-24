# go-vinu upstream security triage through geth v1.17.0

`go-vinu` is based on geth v1.10.8 through Fantom's `develop-1.10.8` line.
It intentionally cherry-picks applicable security fixes instead of rebasing,
because a full geth rebase would mix consensus and EVM changes into VinuChain.

VinuChain's runtime wiring was checked directly: it runs the devp2p server and
registers its custom `opera` protocol plus the fork's `fsnap` protocol. It does
not register geth's `eth` protocol. The dispositions below therefore assume
devp2p and snap peer input is reachable.

| CVE | Upstream fix | Fork disposition |
|---|---:|---|
| CVE-2021-41173 | v1.10.9 | Not applicable: the inherited snap handler already has lookup, byte, and time bounds. |
| CVE-2022-29177 | v1.10.17 | Applied: `DiscReason` is one byte and disconnect messages decode into a one-field list, matching geth PR #24507. |
| CVE-2023-40591 | v1.12.1 | Applied: inbound pings use the single bounded `pingLoop`; `TestPeerPingFlood` covers it. |
| CVE-2024-32972 | v1.13.15 | Not applicable: the vulnerable `GetHeadersFrom(num+count-1, count-1)` optimization does not exist; the inherited loop returns immediately for `Amount == 0`. The geth `eth` protocol is also not registered by VinuChain. |
| CVE-2026-22862 | v1.16.8 | Applied: ECIES rejects ciphertext shorter than the public key, MAC, and AES block before slicing the IV; `TestDecryptRejectsShortCiphertext` covers it. |
| CVE-2026-22868 | v1.16.8 | Not applicable: this fork has no blob transaction implementation or KZG verifier, rejects type `0x03` in both full and light transaction pools, and VinuChain does not use geth's transaction fetcher. |
| CVE-2026-26313 | v1.17.0 | Applied to the reachable snap protocol: response lists and nested trie-node request paths remain raw until their encoded byte and item counts are bounded; focused tests cover each limit. The unregistered geth `eth` handlers need no backport. |
| CVE-2026-26314 | v1.16.9 | Applied on the supported Linux/CGO path: `BitCurve.IsOnCurve` rejects non-canonical coordinates and the C scalar multiplier checks field parsing; focused tests cover coordinates at the field prime. |
| CVE-2026-26315 | v1.16.9 | Applied in `407a9c5ca`: ECIES validates peer coordinates before ECDH; off-curve and nil-coordinate regression tests cover it. |

In the consumer module `VinuChain/VinuChain`, the effective fork version and
the left-hand `github.com/ethereum/go-ethereum` requirement serve different
tools: its `go.mod` uses an explicit `replace` to `github.com/VinuChain/go-vinu`,
while the left-hand version records the audited upstream advisory floor for
Dependabot. This repository itself retains geth's module path and has no such
replacement.

These patches affect unauthenticated network parsing only. They do not change
the EVM, state transition, receipt encoding, chain rules, activation heights,
or protocol capability names.

Release binaries require Linux/CGO. The inherited non-CGO signature backend
does not compile against the current btcec dependency and is not a supported
VinuChain build target; no advisory disposition relies on it.

The generic `FeeRefundActive` torn-root interleaving remains real and covered by
a stress test. It is not reachable in VinuChain's release path: mainnet already
has Podgorica active, so an upgrade starts with the flag true and never changes
it. A fresh activation has one monotonic false-to-true change in the serialized
epoch seal before block dispatch and before any non-zero refund receipt can
exist; all earlier refunds are nil or zero and encode identically. A second
writer, reverse transition, or post-dispatch activation would require a code
fix; the consumer's activation-order tests pin the current invariant.

Two full-package p2p tests (`TestServerSetupConn` and `TestServerPeerLimits`)
remain inherited timing/order flakes. CI runs the deterministic peer security
tests directly; the VinuChain repository remains the full integration gate.
