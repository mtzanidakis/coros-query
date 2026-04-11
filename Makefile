BINARY := coros-query
BIN_DIR := bin
PKG := ./...
LDFLAGS := -s -w

.PHONY: all build test vet fmt tidy run clean

all: build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .

test:
	go test $(PKG)

vet:
	go vet $(PKG)

fmt:
	gofmt -w .

tidy:
	go mod tidy

run: build
	$(BIN_DIR)/$(BINARY) $(ARGS)

clean:
	rm -rf $(BIN_DIR)
