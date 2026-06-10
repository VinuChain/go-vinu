# go-vinu (VinuChain's go-ethereum fork)

VinuChain's fork of [go-ethereum](https://github.com/ethereum/go-ethereum), providing the EVM execution layer, P2P networking, JSON-RPC, and account management for [VinuChain](https://github.com/VinuChain/VinuChain).

The Go module path remains `github.com/ethereum/go-ethereum` for compatibility with the upstream import graph. VinuChain's `go.mod` consumes this fork via a `replace` directive:

```
replace github.com/ethereum/go-ethereum => github.com/VinuChain/go-vinu v1.20.24-quota
```

## Why the fork?

VinuChain requires EVM-level changes that are not applicable upstream. The key modifications:

### FeeRefund (payback system)

VinuChain's fee refund mechanism returns a portion of transaction fees to eligible stakers. This required changes throughout the execution pipeline:

- **`receipt.FeeRefund` field** — added to `core/types.Receipt` to carry refund amounts through the execution and storage pipeline
- **RLP encoding** — FeeRefund is included in receipt RLP serialization with defensive nil handling
- **JSON marshaling** — FeeRefund appears in `eth_getTransactionReceipt` RPC responses
- **GraphQL** — `feeRefund` field added to the Transaction schema

> The consensus inclusion of `FeeRefund` is gated by the process-global
> `FeeRefundActive` flag (activated at the Podgorica fork). See
> [FORK.md](FORK.md) §2.2 and [SECURITY-TRIAGE.md](SECURITY-TRIAGE.md) for the
> safety contract (set-once-before-start, never-flip).

### EVM hard-fork backports

Shanghai, a Cancun **subset**, and Prague EVM features have been backported and
gated behind VinuChain-specific fork switches (`VinuBLSBlock`,
`VinuLatestEVMBlock` in `params.ChainConfig`):

- **Shanghai:** PUSH0 (EIP-3855), warm coinbase (EIP-3651), initcode limits (EIP-3860).
- **Cancun (subset):** transient storage TLOAD/TSTORE (EIP-1153), MCOPY (EIP-5656),
  SELFDESTRUCT-only-same-tx (EIP-6780). **Not included:** BLOBHASH/BLOBBASEFEE,
  blob transactions (EIP-4844), and the beacon-root contract (EIP-4788); blob txs
  are explicitly rejected in the pool.
- **Prague:** set-code transactions (EIP-7702), per-tx gas cap (EIP-7825),
  BLS12-381 (EIP-2537) and P256 precompiles.

See [FORK.md](FORK.md) for the full delta and upstream-tracking policy.

### Security hardening (audit fixes)

- **btcec v1 to v2 migration** — replaced `btcsuite/btcd` v1 with `btcd/btcec/v2` (v2.3.6) to resolve signature malleability vulnerabilities in secp256k1 operations
- **FeeRefund pointer safety** — fixed pointer aliasing that could leak values across receipts in the same block
- **Per-connection subscription limits** — prevent resource exhaustion from unbounded WebSocket/IPC subscriptions
- **JS tracer hardening** — fixed call depth tracking to prevent crashes, disabled source map loading in the goja JS engine
- **Removed docker/docker dependency** — replaced `docker/pkg/reexec` with internal implementation, eliminating CVE alerts (GHSA-x744-4wpc-v9h2, GHSA-pxq6-2prw-chj9)

### Modernization

- Go directive bumped from 1.15 to 1.25.8
- Removed deprecated `fjl/memsize` dependency
- Updated `golang.org/x/*` packages for compatibility
- Console banner rebranded from Vinu to VinuChain

## What is NOT changed

This fork preserves most of the go-ethereum API surface; standard JSON-RPC
methods, account management, and developer tools are largely unchanged. The EVM,
however, is **not** identical to upstream:

- The Cancun support is a **subset** — `BLOBHASH`/`BLOBBASEFEE` and EIP-4844 blob
  transactions are **not** implemented (blob txs are rejected). Contracts compiled
  with `evm_version = cancun` that use those opcodes will hit invalid opcodes.
- EIP-6780 "created in this transaction" tracking is broader than upstream (see
  [FORK.md](FORK.md) §2.3); this is a documented semantic divergence.

The p2p/sync layers are inherited unmodified from the geth v1.10.8 base; see
[SECURITY-TRIAGE.md](SECURITY-TRIAGE.md) for the post-1.10.8 CVE dispositions.

## Building

```shell
make geth        # builds ./build/bin/geth via `go build`
# or directly:
go build ./...
```

Requires Go 1.25+ (see `go.mod`) and a C compiler. The build output is
`build/bin/geth`, though VinuChain nodes consume this as a **library**, not as a
standalone binary. (The historical `build/ci.go` helper was removed; the Makefile
now uses plain `go build`/`go test`/`go vet`.)

## Testing

```shell
make test-fork   # CI-gated fork/consensus packages (recommended)
make test        # full module suite (some upstream-inherited packages may fail)
```

CI runs `make`-equivalent build + vet + the fork-touched test packages on every
push and pull request — see [`.github/workflows/ci.yml`](.github/workflows/ci.yml).

## Versioning

Tags follow the pattern `v1.20.X-quota` to indicate the VinuChain quota/payback feature branch. The current tip is **`v1.20.24-quota`**; run `git tag | grep quota` for the full list and see [FORK.md](FORK.md) for the delta.

> **Branch caveat:** the live fork delta is on branch `elemont`. The GitHub
> default branch `master` is an ancient geth-1.9.6-era snapshot and does not
> reflect production code — build, audit, and integrate against `elemont`.

| Tag | Description |
|-----|-------------|
| `v1.20.9-quota` → `v1.20.24-quota` | 2026 hardening + EVM-backport campaign: Shanghai/Cancun-subset/Prague behind VinuBLS/VinuLatestEVM switches, RPC/crypto hardening, CI (see [FORK.md](FORK.md)) |
| `v1.20.8-quota` | Remove docker/docker CVE dependency, Go 1.25.8, full dep alignment |
| `v1.20.7-quota` | Dependency alignment: Go 1.25.8, golang.org/x/*, protobuf |
| `v1.20.6-quota` | Audit hardening, btcec v2, Go 1.24.1 |
| `v1.20.5-quota` | FeeRefund pointer fixes, subscription limits |
| `v1.20.4-quota` | btcec v2 migration |
| `v1.20.3-quota` | Go 1.24.1 upgrade |
| `v1.20.2-quota` | JS tracer fixes, goja upgrade |
| `v1.20.1-quota` | FeeRefund field, RPC/GraphQL support |
| `v1.20.0-quota` | Initial FeeRefund implementation |

## Upstream

Based on go-ethereum. The original go-ethereum README and documentation can be found at [geth.ethereum.org](https://geth.ethereum.org).

## License

GNU Lesser General Public License v3.0 (library code) and GNU General Public License v3.0 (binaries) — same as upstream go-ethereum. See [COPYING.LESSER](COPYING.LESSER) and [COPYING](COPYING).
