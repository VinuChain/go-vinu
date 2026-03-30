# go-vinu (VinuChain's go-ethereum fork)

VinuChain's fork of [go-ethereum](https://github.com/ethereum/go-ethereum), providing the EVM execution layer, P2P networking, JSON-RPC, and account management for [VinuChain](https://github.com/VinuChain/VinuChain).

The Go module path remains `github.com/ethereum/go-ethereum` for compatibility with the upstream import graph. VinuChain's `go.mod` consumes this fork via a `replace` directive:

```
replace github.com/ethereum/go-ethereum => github.com/VinuChain/go-vinu v1.20.6-quota
```

## Why the fork?

VinuChain requires EVM-level changes that are not applicable upstream. The key modifications:

### FeeRefund (payback system)

VinuChain's fee refund mechanism returns a portion of transaction fees to eligible stakers. This required changes throughout the execution pipeline:

- **`receipt.FeeRefund` field** — added to `core/types.Receipt` to carry refund amounts through the execution and storage pipeline
- **RLP encoding** — FeeRefund is included in receipt RLP serialization with defensive nil handling
- **JSON marshaling** — FeeRefund appears in `eth_getTransactionReceipt` RPC responses
- **GraphQL** — `feeRefund` field added to the Transaction schema

### Security hardening (audit fixes)

- **btcec v1 to v2 migration** — replaced `btcsuite/btcd` v1 with `btcd/btcec/v2` (v2.3.6) to resolve signature malleability vulnerabilities in secp256k1 operations
- **FeeRefund pointer safety** — fixed pointer aliasing that could leak values across receipts in the same block
- **Per-connection subscription limits** — prevent resource exhaustion from unbounded WebSocket/IPC subscriptions
- **JS tracer hardening** — fixed call depth tracking to prevent crashes, disabled source map loading in the goja JS engine

### Modernization

- Go directive bumped from 1.15 to 1.24.1
- Removed deprecated `fjl/memsize` dependency
- Updated `golang.org/x/*` packages for compatibility
- Console banner rebranded from Vinu to VinuChain

## What is NOT changed

This fork preserves the full go-ethereum API surface. All standard Ethereum JSON-RPC methods, EVM opcodes, P2P protocols, account management, and developer tools work identically to upstream go-ethereum. The fork only adds VinuChain-specific extensions.

## Building

```shell
make geth
```

Requires Go 1.22+ and a C compiler. The build output is `build/bin/geth`, though VinuChain nodes use this as a library — not as a standalone binary.

## Testing

```shell
go test ./...
```

## Versioning

Tags follow the pattern `v1.20.X-quota` to indicate the VinuChain quota/payback feature branch:

| Tag | Description |
|-----|-------------|
| `v1.20.6-quota` | Latest: audit hardening, btcec v2, Go 1.24.1 |
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
