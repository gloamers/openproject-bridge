.PHONY: build test test-integration tidy check check-quick check-no-integration fmt

# Prefer Task (Taskfile.yml). Makefile is a thin shim for parity.
build:
	task build

test:
	task test

test-integration:
	task test:integration

tidy:
	task tidy

fmt:
	task fmt

check:
	task check

check-quick:
	task check:quick

check-no-integration:
	task check:no-integration
