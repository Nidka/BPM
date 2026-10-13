AIR_VERSION := v1.67.4
AIR_BIN := .tools/bin/air
AIR_STAMP := .tools/.air-$(AIR_VERSION)

.PHONY: run dev tools test test-race fmt vet check db-up db-down

run:
	go run ./cmd/api

dev: tools
	$(AIR_BIN) -c .air.toml

tools: $(AIR_STAMP)

$(AIR_STAMP):
	mkdir -p .tools/bin
	GOBIN=$(CURDIR)/.tools/bin go install github.com/air-verse/air@$(AIR_VERSION)
	touch $(AIR_STAMP)

test:
	go test ./...

test-race:
	go test -race ./...

fmt:
	gofmt -w $$(find cmd internal -name '*.go' -type f)

vet:
	go vet ./...

check: fmt vet test

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

