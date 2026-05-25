# Makefile для mygrep — собирает бинарники в ./bin и помогает запустить
# демонстрационные сценарии.

GO        ?= go
BIN_DIR   := bin
SERVER_BIN := $(BIN_DIR)/mygrep-server
CLIENT_BIN := $(BIN_DIR)/mygrep

.PHONY: all build server client test vet fmt clean run compare bench tidy

all: build

build: $(SERVER_BIN) $(CLIENT_BIN)

$(SERVER_BIN): $(shell find cmd/mygrep-server internal -name '*.go') go.mod
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(SERVER_BIN) ./cmd/mygrep-server

$(CLIENT_BIN): $(shell find cmd/mygrep internal -name '*.go') go.mod
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(CLIENT_BIN) ./cmd/mygrep

server: $(SERVER_BIN)
client: $(CLIENT_BIN)

test:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

run: build
	./scripts/run_local.sh

compare: build
	./scripts/compare_with_grep.sh

bench: build
	./scripts/bench.sh

clean:
	rm -rf $(BIN_DIR)
