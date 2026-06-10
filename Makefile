# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

# NOTE (VinuChain fork): the upstream `build/ci.go` helper was removed from this
# fork (the directory `build/` was deleted), so the historical targets that ran
# `go run build/ci.go ...` no longer work. The targets below are rewritten as
# plain `go build`/`go test`/`go vet` invocations. This repo is consumed as a
# library by the VinuChain node; `cmd/geth` is built for convenience only and is
# not the production binary (see README.md).

.PHONY: geth all test test-fork vet lint clean

GOBIN = ./build/bin

geth:
	go build -o $(GOBIN)/geth ./cmd/geth
	@echo "Done building."
	@echo "Run \"$(GOBIN)/geth\" to launch geth."

all:
	go build ./...

# test runs the full module test suite. Some upstream-inherited packages may be
# slow or flaky; for the fork-critical, CI-gated subset use `make test-fork`.
test:
	go test ./...

# test-fork mirrors the CI allow-list: the consensus/fork-touched packages that
# are known-green on this toolchain (see .github/workflows/ci.yml).
test-fork:
	go test ./core/ ./core/types/... ./core/vm/... ./core/state/... \
		./rpc/... ./internal/ethapi/... ./crypto/...

vet:
	go vet ./...

lint: vet ## Run linters (go vet).

clean:
	go clean -cache
	rm -fr $(GOBIN)/*

# The devtools target installs tools required for 'go generate'.
# You need to put $GOBIN (or $GOPATH/bin) in your PATH to use 'go generate'.

devtools:
	env GOBIN= go install golang.org/x/tools/cmd/stringer@latest
	env GOBIN= go install github.com/kevinburke/go-bindata/go-bindata@latest
	env GOBIN= go install github.com/fjl/gencodec@latest
	env GOBIN= go install github.com/golang/protobuf/protoc-gen-go@latest
	env GOBIN= go install ./cmd/abigen
	@type "solc" 2> /dev/null || echo 'Please install solc'
	@type "protoc" 2> /dev/null || echo 'Please install protoc'
