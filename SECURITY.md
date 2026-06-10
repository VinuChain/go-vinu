# Security Policy

This is **go-vinu**, the VinuChain execution-layer fork of go-ethereum. It is
consumed as a library by the VinuChain node (chain ID 206). Vulnerabilities in
this fork should be reported to **VinuChain**, not to the upstream
ethereum/go-ethereum bug-bounty program.

## Supported Versions

The supported code is on branch `elemont` (current tag `v1.20.24-quota`). The
GitHub default branch `master` is an ancient (geth-1.9.6-era) snapshot and is
**not** maintained — do not report or audit against it.

## Reporting a Vulnerability

**Please do not file a public issue or PR describing the vulnerability.**

Report security issues privately to the VinuChain team via:

- GitHub private vulnerability reporting on this repository
  (`Security` → `Report a vulnerability`), or
- the VinuChain security contact listed at <https://vinuchain.org>.

Please include the affected version/commit, a description, and reproduction
steps or a proof-of-concept where possible.

## Scope notes

- **Fork-specific consensus surface** (highest priority): the `FeeRefund`
  receipt field and its `FeeRefundActive` gating, the VinuBLS/VinuLatestEVM EVM
  backports, and the JSON-RPC hardening. See [FORK.md](FORK.md) and
  [SECURITY-TRIAGE.md](SECURITY-TRIAGE.md).
- **Inherited upstream code** (p2p/sync/discovery from the geth v1.10.8 base):
  post-1.10.8 upstream advisories are triaged in
  [SECURITY-TRIAGE.md](SECURITY-TRIAGE.md). Fixes are adopted into this fork by
  cherry-pick, never rebase.

## Upstream reference

This fork is based on go-ethereum. Upstream's security policy and bounty program
(which do **not** cover this fork) are documented at
<https://geth.ethereum.org/docs/developers/geth-developer/disclosures> and
<https://bounty.ethereum.org>. Consult them for vulnerabilities in unmodified
upstream geth code, but report through the VinuChain channels above.
