# Makefile for policycheck

.PHONY: build build-scanner lint clean test

build: build-scanner
	go build ./...

build-scanner:
	npm run build:scanner

lint:
	golangci-lint run
	ruff check cmd/policycheck/policy_scanner.py
	npx tsc --noEmit cmd/policycheck/policy_scanner.ts

clean:
	rm -f policycheck
	rm -f cmd/policycheck/policycheck
	rm -rf .policycheck/exports/*

test:
	go test ./...
