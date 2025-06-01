BINARY_NAME=nanokvm-redfish
GOARCH=riscv64
GOOS=linux
GO=go

.PHONY: build
build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 $(GO) build -o $(BINARY_NAME) -ldflags="-s -w" main.go

.PHONY: clean
clean:
	rm -f $(BINARY_NAME)

.PHONY: run
run:
	$(GO) run main.go

.PHONY: test
test:
	$(GO) test ./...

.PHONY: test-coverage
test-coverage:
	$(GO) test -cover ./...

.PHONY: fmt
fmt:
	$(GO) fmt ./...