# go-vinu — Post-1.10.8 Upstream Security Triage

Scope: published `ethereum/go-ethereum` security advisories with a fixed version
**≥ v1.10.9**, triaged against this fork's actual code. Per the upstream-tracking
policy ([FORK.md](./FORK.md) §3) the approach is **cherry-pick, never rebase**.

Base: geth **v1.10.8** via Fantom's `develop-1.10.8` (merge-base `39f2942b`,
2023-07-28). Note that Fantom's 1.10.8 line already absorbed some upstream
backports, so a CVE "fixed after 1.10.8 upstream" is not automatically present
here — each is checked against the code.

Method: for each CVE, locate the vulnerable code path; verify whether it is
present and reachable in this fork; assign a verdict.

---

## Threat-model gate (the load-bearing open question)

Every p2p/discovery CVE below is **conditional on the VinuChain node actually
running this repo's devp2p server / discovery**. This is an Opera/Lachesis-style
consumer that uses a custom wire protocol and consumes go-vinu primarily as an
**EVM/state/types/RPC library**. Two possibilities:

- **If the node does NOT run `p2p.Server` / discv4 / discv5 / the eth & snap
  protocol handlers**, then C1–C4 are all *not reachable* regardless of the
  code, and only the RPC-surface items matter.
- **If the node DOES run the devp2p server** (typical for full nodes even with
  custom protocols), the reachable items below apply.

This cannot be determined from this repo. **Owner action:** confirm the node's
wiring. The cherry-pick applied below (C3) is safe and low-cost either way; the
rest of the dispositions assume the worst case (devp2p server is running) so
that "not applicable" verdicts rest on code facts, not on the assumption that
the server is off.

---

## Disposition table

| ID | CVE | Fixed in | Class | Reachable here? | Verdict |
|----|-----|----------|-------|-----------------|---------|
| C1 | CVE-2021-41173 | v1.10.9 | snap/1 `GetTrieNodes` unbounded serve (DoS) | No — fix already in base | **Not applicable** |
| C2 | CVE-2022-29177 | v1.10.17 | discv5 crafted packet crash under high-verbosity logging | Conditional (discv5 + TRACE/DEBUG logging) | **Needs decision** |
| C3 | CVE-2023-40591 | v1.12.1 | devp2p ping handler spawns unbounded goroutines (DoS) | Yes (if devp2p server runs) | **Applied (cherry-pick #27887)** |
| C4 | CVE-2024-32972 | v1.13.15 | eth `GetBlockHeadersRequest count=0` underflow (DoS) | No — vulnerable function not present | **Not applicable** |

---

## C1 — CVE-2021-41173 (snap/1 GetTrieNodes) — NOT APPLICABLE

**Vulnerability:** prior to v1.10.9, the snap protocol `GetTrieNodes` handler
could be made to serve an unbounded number of trie nodes / spend unbounded time,
crashing or stalling the node.

**Check (code fact):** `eth/protocols/snap/handler.go` already enforces
`maxTrieNodeLookups = 1024`, `maxTrieNodeTimeSpent = 5*time.Second`, and a
per-iteration `bytes > req.Bytes || loads > maxTrieNodeLookups ||
time.Since(start) > maxTrieNodeTimeSpent` break (handler.go:500, 506). These
exact bounds are **already present in the 1.10.8 merge-base**
(`git show 39f2942b:eth/protocols/snap/handler.go` shows them at the same lines).
Fantom's 1.10.8 line was cut after this fix was backported upstream.

**Verdict:** Not applicable — the fix is already in the base. No action.

---

## C2 — CVE-2022-29177 (discv5 crafted-packet crash) — NEEDS DECISION

**Vulnerability:** prior to v1.10.17, a node **configured for high-verbosity
logging** (TRACE/DEBUG) could be crashed by a specially crafted discv5 p2p
message. Workaround per the upstream advisory: run at the default `INFO`
verbosity, which makes the node not vulnerable.

**Check (code fact):** discv5 is present and reachable
(`p2p/discover/v5wire/`, `p2p/discover/v5_udp.go`); `handlePacket`
(v5_udp.go:657) logs decoded packet names at `Trace`/`Debug`. The crash path is
in the verbose logging of malformed packets.

**Why not auto-applied:** (a) severity is Low and the trigger requires a
**non-default** logging configuration; (b) the precise upstream fix could not be
pinned to an exact, cleanly-cherry-pickable diff against this fork's v5wire code
during triage, so applying it blind would violate the "only clean, verified
cherry-picks" rule. Applying an unverified packet-parsing patch to consensus-
adjacent networking code is higher risk than the bug it would fix.

**Owner action / decision needed:**
1. Confirm production log verbosity. If the node runs at `INFO` (default), the
   bug is **not triggerable** and this can be closed as accepted.
2. If high-verbosity logging is used (or discv5 is exposed), pin the upstream
   v1.10.17 v5wire fix commit and cherry-pick it with a malformed-packet
   regression test.

**Verdict:** Needs decision (conditional, mitigated by default config).

---

## C3 — CVE-2023-40591 (ping goroutine flood) — APPLIED

**Vulnerability:** the devp2p base-protocol ping handler responded to each
inbound `pingMsg` by spawning a fresh goroutine (`go SendItems(p.rw, pongMsg)`).
An attacker flooding a peer connection with pings could create an unbounded
number of goroutines, exhausting memory (High, fixed upstream v1.12.1, PR
ethereum/go-ethereum#27887).

**Check (code fact):** the exact vulnerable line was present at
`p2p/peer.go:325`:

```go
case msg.Code == pingMsg:
    msg.Discard()
    go SendItems(p.rw, pongMsg)   // <-- unbounded goroutine per ping
```

reachable from `Peer.handle` whenever this repo's `p2p.Server` runs.

**Fix applied (cherry-pick of #27887):** added a buffered `pingRecv chan
struct{}` (cap 16) to `Peer`; the ping handler now signals the existing single
`pingLoop` goroutine instead of spawning one per ping; `pingLoop` sends the pong
synchronously. This bounds the work a ping flood can induce to the existing,
fixed set of per-peer goroutines.

Files: `p2p/peer.go` (struct field, constructor init, ping handler, pingLoop
case). Regression test: `p2p/peer_test.go::TestPeerPingFlood` (sends 64 pings,
asserts 64 pongs; passes under `-race`).

**Verdict:** Applied. `go build ./p2p/...`, `go vet ./p2p/`, and the peer/ping
tests (`TestPeerPing`, `TestPeerPingFlood`, `TestPeerDisconnect*`) are green,
including under `-race`.

> Note: the p2p package has two **pre-existing, fork-inherited** test failures
> (`TestServerSetupConn`, `TestServerPeerLimits`) that fail identically on the
> unmodified base and are unrelated to this change. They are the reason the p2p
> package is not in the CI test allow-list; they are recorded here as an owner
> follow-up, not introduced by this work.

---

## C4 — CVE-2024-32972 (GetBlockHeadersRequest count=0 underflow) — NOT APPLICABLE

**Vulnerability:** prior to v1.13.15, the eth protocol header server used an
optimized reverse-batch read `chain.GetHeadersFrom(num+count-1, count-1)`. A
`GetBlockHeadersRequest` with `count == 0` made `count-1` underflow to
`UINT64_MAX`, requesting all headers back to genesis and exhausting memory
(High, fixed PR ethereum/go-ethereum#29534).

**Check (code fact):** the vulnerable `GetHeadersFrom` optimization **does not
exist** in this fork. Header serving uses the older 1.10.8 bounded loop
`answerGetBlockHeadersQuery` (`eth/protocols/eth/handlers.go:52`), which iterates
under `len(headers) < int(query.Amount)`, `bytes < softResponseLimit`,
`len(headers) < maxHeadersServe`, and `lookups < 2*maxHeadersServe` guards. With
`query.Amount == 0` the loop body never executes (no underflow). The reverse/
skip arithmetic also has explicit overflow guards (handlers.go:93, 105).

**Verdict:** Not applicable — the vulnerable function was never introduced into
this base; the inherited loop is bounded.

---

## FeeRefundActive consensus coupling (audit H1) — cross-reference

Not an upstream CVE, but the highest consensus hazard. The process-global
`FeeRefundActive atomic.Bool` (`core/types/receipt.go`) gates receipt-root
encoding. Characterization tests in
`core/types/fee_refund_epoch_test.go` prove that under a **concurrent flag
flip**, `DeriveSha(Receipts)` can yield a receipt root matching neither epoch
(per-index `EncodeIndex` reads the live flag). The flag access is race-free; the
hazard is *logical epoch incorrectness*, not a data race.

**Safe operating contract (must be honored by the consumer node):**
`FeeRefundActive` MUST be set exactly once, before any block import / replay /
RPC receipt derivation begins, and MUST NOT be flipped while the process is live.
If the consumer instead flips it at the Podgorica block boundary mid-process,
historical receipt re-encodes (sync, re-org, replay, RPC) become
non-deterministic. **Owner action:** confirm the consumer sets the flag at
process start (config-driven), not at the fork block; consider a one-way
startup-only setter + assertion in both repos.

---

## Owner action summary

1. **Confirm devp2p wiring** — does the node run `p2p.Server` / discv4 / discv5 /
   eth & snap handlers? This gates C2 and the reachability of C3.
2. **C2** — confirm production log verbosity; if not `INFO`, pin & cherry-pick
   the v1.10.17 discv5 fix with a regression test.
3. **FeeRefundActive** — confirm set-once-at-startup contract in the consumer.
4. **p2p pre-existing test failures** (`TestServerSetupConn`,
   `TestServerPeerLimits`) — triage separately; inherited from the base.
