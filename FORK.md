# go-vinu — Fork Documentation

`go-vinu` is the VinuChain execution-layer fork of go-ethereum. It is consumed
**as a library** by the VinuChain node (an Opera/Lachesis-style consumer, chain
ID 206) for the EVM, state, receipt/transaction types, and JSON-RPC. The
`cmd/geth` binary still builds but is **not** the production binary.

The Go module path `github.com/ethereum/go-ethereum` is intentionally retained
so downstream `replace` directives keep working.

---

## 1. Fork base

| | |
|---|---|
| Upstream lineage | `ethereum/go-ethereum` → `Fantom-foundation/go-ethereum` (Opera/Lachesis) → `VinuChain/go-vinu` |
| Base version | **geth v1.10.8** (`params/version.go`: 1.10.8-stable) |
| Merge-base | `39f2942b5902c9dbe4d4a74dea5a50f7e08380c4` on `Fantom-foundation/go-ethereum` `develop-1.10.8` (2023-07-28) |
| Toolchain | Go 1.25.8 (`go.mod`) |

The fork remotes are:
- `origin` = `https://github.com/VinuChain/go-vinu.git`
- `upstream` = `https://github.com/Fantom-foundation/go-ethereum.git`

> **Default branch caveat:** the live fork delta lives on branch `elemont`
> (tagged `v1.20.24-quota`). The GitHub default branch `master` is an ancient
> geth-1.9.6-era snapshot and does **not** reflect production code. Audit and
> integrate against `elemont`. (Audit finding M4.)

Fork delta vs. the merge-base: ~63 commits, 147 files, +7,626 / −5,757 lines, of
which ~36 test files account for ~3,206 lines.

---

## 2. The VinuChain delta

History shape: the original FeeRefund work landed 2023–2024; an intensive
audit-driven hardening + EVM-backport campaign followed in 2026-03 → 2026-06.

### 2.1 FeeRefund — the consensus-visible receipt field

VinuChain adds a `FeeRefund *big.Int` field to `types.Receipt`
(`core/types/receipt.go`). It is part of the **consensus** RLP encoding
(`receiptRLP`), the storage encoding (`storedReceiptRLP`), the JSON marshaling,
the GraphQL schema, and the `eth_getTransactionReceipt` RPC response.

`FeeRefund` values are populated by the **consumer node**, not by this repo.

### 2.2 `FeeRefundActive` — process-global epoch gating

```go
// core/types/receipt.go
var FeeRefundActive atomic.Bool
```

This flag gates whether non-zero `FeeRefund` is included in the consensus receipt
encoding:

- **Encode** (`feeRefundForEncoding`, used by `Receipt.EncodeRLP` and
  `Receipts.EncodeIndex`): when the flag is `false`, the encoded `FeeRefund` is
  forced to zero, so pre-Podgorica receipt roots are deterministic regardless of
  any stray value on the struct.
- **Decode** (`setFromRLP`): when the flag is `false`, a peer-supplied non-zero
  `FeeRefund` is discarded; when `true`, it is accepted (with a 32-byte size cap
  to bound heap allocation).

The contract is that the consumer sets `FeeRefundActive` to `true` at the
**Podgorica** activation. **No non-test code in this repo ever calls
`.Store(...)`** — the setter lives in the consumer node. See
[`SECURITY-TRIAGE.md`](./SECURITY-TRIAGE.md) §FeeRefundActive (or the audit-impl
report) for the safety analysis of this temporal coupling (audit finding H1).

### 2.3 EVM hard-fork backports (behind Vinu-specific switches)

Shanghai / Cancun(-subset) / Prague EVM features were re-implemented locally and
gated by VinuChain fork switches in `params.ChainConfig`:

- `VinuBLSBlock` — activates the BLS12-381 / EIP-2537 precompile set.
- `VinuLatestEVMBlock` — activates the "latest EVM" instruction/precompile set.

Plumbed through `ChainConfig` String/ordering checks, `CheckCompatible`, and
`Rules` (`params/config.go`).

Feature backports include:
- **Shanghai:** PUSH0 (EIP-3855), warm coinbase (EIP-3651), initcode limits
  (EIP-3860).
- **Cancun (subset):** transient storage TLOAD/TSTORE (EIP-1153), MCOPY
  (EIP-5656), SELFDESTRUCT-only-same-tx (EIP-6780).
  **NOT included:** BLOBHASH / BLOBBASEFEE / blob transactions (EIP-4844) and
  the beacon-root contract (EIP-4788). Blob txs are explicitly rejected in the
  pool. (Audit finding M2 — README's "identical to upstream" claim is therefore
  inaccurate.)
- **Prague:** EIP-7702 set-code transactions (`SetCodeTx`), the per-tx gas cap
  (EIP-7825), and BLS12-381 / P256 precompiles.

Supporting state machinery: transient storage (`core/state/transient_storage.go`)
and `createdObjects` tracking for EIP-6780 (`core/state/statedb.go`).

> **EIP-6780 note (audit M5):** `CreatedInThisTransaction` is backed by
> `createdObjects`, which is populated for *every* newly instantiated state
> object including fresh EOAs — broader than upstream's CREATE/CREATE2/tx-create
> marking. This is a semantic-equivalence divergence, not a crash; it must be
> settled (and gated) before Cancun activates on mainnet.

### 2.4 Security / RPC hardening (2026 campaign)

- JSON-RPC batch cap (100 requests) and in-process RPC concurrency limit
  (default 50); WebSocket semaphore removed; `Stop` race fixed.
- `StateOverride` caps: reject oversized code blobs, cap `StateDiff` at 1000
  entries.
- `eth/filters`: enforce topic-criteria limits in `UnmarshalJSON`.
- crypto: `secp256k1.IsOnCurve` coordinate-range `[0, P)` check; ECIES peer
  public-key on-curve validation before ECDH; btcec v1→v2 migration
  (signature-malleability fix).
- JS tracer (goja): call-depth tracking / `CaptureExit` crash guards; source-map
  loading disabled.
- deps: `docker/docker` (CVE-bearing) replaced with an internal reexec package;
  toolchain and crypto/net libraries modernized.
- `Transaction.From()`: removed the silent HomesteadSigner fallback.

### 2.5 Other fork-relevant surface

- `core/vm/evm.go`: `BaseFeeFloor` pass-through field in `BlockContext` for
  congestion detection.
- `build/` directory deleted; the Makefile and CI were rewired to plain
  `go build`/`go test`/`go vet` (audit M3, fixed in the audit-impl branch).

---

## 3. Upstream-tracking policy

**Cherry-pick, never rebase.** A 1.10.8 → current rebase would forfeit ~5 years
of fork validation (Lachesis integration, the locally re-implemented EVM
backports, the 2026 hardening) for marginal gain and is explicitly **not**
recommended.

Policy:

1. **EVM / consensus features** are backported deliberately and gated behind a
   VinuChain fork switch (e.g. `VinuBLSBlock`, `VinuLatestEVMBlock`), with tests,
   never by pulling an upstream hard fork wholesale.
2. **Security fixes** from upstream geth releases ≥ v1.10.9 are triaged
   individually: each CVE gets a written disposition (applicable / not
   applicable / needs-decision) in [`SECURITY-TRIAGE.md`](./SECURITY-TRIAGE.md),
   and only clearly-applicable + cleanly-cherry-pickable fixes are applied, with
   an upstream commit reference and tests.
3. **p2p / sync / discovery** code is inherited unmodified from the 1.10.8 base.
   Whether the VinuChain node actually runs this repo's devp2p server determines
   whether the inherited p2p DoS CVEs apply at all; see `SECURITY-TRIAGE.md`.
4. **CI gates merges:** every push/PR runs build + vet + the fork-touched test
   packages (`.github/workflows/ci.yml`). Consensus-sensitive code merges only
   through a green gate.
5. **Out of scope** (deliberately not churned): `mobile/`, `swarm/` (stub),
   `les/` beyond the gas-cap touch — diff noise for zero consensus value.

### Version identity

`params/version.go` currently reports `1.10.8-stable` (surfaced via
`web3_clientVersion`) while release tags reach `v1.20.24-quota` (audit L2). The
base-version number documents the geth lineage; the `-quota` tag documents the
VinuChain release. These should be reconciled into a single Vinu-identifiable
client-version string.
