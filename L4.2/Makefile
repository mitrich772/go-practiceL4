# Makefile для mygrep.
# Главное:
#   make up        — поднять 3 сервера в docker
#   make down      — погасить
#   make build     — собрать клиент в ./bin/mygrep
#   make test      — unit-тесты
#   make compare   — сравнение mygrep с системным grep

GO        ?= go
BIN_DIR   := bin
CLIENT_BIN := $(BIN_DIR)/mygrep

SERVERS := 127.0.0.1:9101,127.0.0.1:9102,127.0.0.1:9103

.PHONY: build up down restart compare test vet fmt tidy clean ps logs

build: $(CLIENT_BIN)

$(CLIENT_BIN): $(shell find cmd/mygrep internal -name '*.go') go.mod
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(CLIENT_BIN) ./cmd/mygrep

up:
	docker compose up -d --wait --build server1 server2 server3

down:
	docker compose down -v

restart: down up

ps:
	docker compose ps

logs:
	docker compose logs -f

compare: build
	./scripts/compare_with_grep.sh

test:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BIN_DIR)
