---
name: go-backend-style-mitrich
description: Mitrich's preferred style for writing Go backend services. Use this when starting a new Go HTTP/Kafka/RabbitMQ service or refactoring an existing one in mitrich772's projects (go-practiceL3 L3.3 is the gold-standard reference). Covers project layout, chi+slog stack, cleanenv config, layered architecture (handlers → service → repo/cache), per-handler verb-er interfaces, golang-migrate setup, gomock+testify tests, graceful shutdown and Kafka worker idempotency.
---

# Go Backend Style — Mitrich (`mitrich772`)

Эталонный стиль для Go backend сервисов в проектах Mitrich. **Главный референс — `L3.3/services/commenttree`** в `mitrich772/go-practiceL3`. Дополнительные эталоны: `L3.2/services/shortener` (service-слой + тесты + Redis-кэш), `L3.4` (Kafka worker + graceful shutdown).

Когда применять: любая новая Go-служба mitrich-а (HTTP API, фоновый воркер, бот) в стиле `go-practiceL3`. Если репозиторий — `mitrich772/ClearFly`, не натягивай этот скилл насильно: там другой стек (Gin + sqlx + gateway-микросервисы) и свои конвенции — сверяйся с существующим кодом.

> **Терминология:** в L3.3 слой доступа к БД назывался `store`. В новом коде стандарт — `repo` (чтобы не путать с `storage`). В этом скилле всюду пишу `repo` — он эквивалентен `store` из L3.3.

---

## 1. Стек

| Слой | Библиотека |
|---|---|
| Router | `github.com/go-chi/chi/v5` + `chi/v5/middleware` |
| Logger | `log/slog` (stdlib) — Text для local, JSON для dev/prod |
| Config | `github.com/ilyakaznacheev/cleanenv` (YAML + env tags) |
| DB | `database/sql` + `github.com/lib/pq`; в L3.4 — `github.com/wb-go/wbf/dbpg` |
| Migrations | `github.com/golang-migrate/migrate/v4` (file source, postgres driver) |
| Cache | Redis через `github.com/wb-go/wbf/redis` |
| Kafka | `github.com/wb-go/wbf/kafka` (+ `retry.Strategy`) |
| RabbitMQ | `github.com/wb-go/wbf/rabbitmq` |
| Tests | `github.com/golang/mock` или `go.uber.org/mock` + `github.com/stretchr/testify` |
| ID | `github.com/matoous/go-nanoid/v2` (короткие алиасы), `crypto/rand`+hex (uuid-like 16 байт), `github.com/google/uuid` |
| Validation | `github.com/go-playground/validator/v10` (опционально, в shortener) |

Go версия: **1.25.x**. Не понижай.

---

## 2. Структура проекта (per-service)

```
services/<service>/
├── cmd/
│   ├── <service>/main.go         # entrypoint сервиса
│   └── migrate/migrations.go     # CLI-мигратор (-action up|down|step -n N)
├── config/
│   └── local.yaml
├── internal/
│   ├── config/config.go          # Config + MustLoad
│   ├── handlers/
│   │   ├── handler.go            # struct Handler + New(...) с зависимостями
│   │   ├── <verb>_<entity>.go    # ОДИН эндпоинт = ОДИН файл (стиль L3.3)
│   │   ├── dto/                  # request/response DTO (как в shortener)
│   │   └── validation/           # отдельные валидаторы DTO
│   ├── service/
│   │   ├── <name>.go             # бизнес-логика (опционально для тонких CRUD)
│   │   └── errors.go             # ErrXxx сервисного слоя
│   ├── repo/                     # стандарт. legacy-имя `store/` встречается в L3.3 — допустимо в старом коде
│   │   ├── repo.go               # interface + //go:generate mockgen + sentinel ErrNotFound
│   │   └── postgres/
│   │       └── postgres.go       # impl
│   ├── cache/                    # если нужен кэш
│   │   ├── cache.go              # interface
│   │   └── redis/redis.go
│   ├── middleware/
│   │   └── logger/logger.go      # slog request-logger (chi middleware)
│   └── dto/                      # сквозные DTO (L3.3 кладёт сюда, L3.2 — в handlers/dto)
├── migrations/
│   ├── 0001_init.up.sql
│   └── 0001_init.down.sql
├── web/
│   ├── index.html                # простой тестовый UI на vanilla JS
│   └── app.js
├── README.md
├── go.mod
└── go.sum
```

Нумерация миграций: **`0001_init.up.sql` / `0001_init.down.sql`** (4 цифры). Не используй `000001_…`.

**Дефолт — один сервис = один репозиторий со своим `go.mod`.** Не плоди многомодульные монорепо без необходимости — это усложняет проект.

Если всё-таки два связанных сервиса (API + worker, делящие DTO) живут в одном репо — можно использовать **go workspace** (`go.work`) с отдельным модулем `services/contracts` (DTO/model/interfaces, без логики). Так сделано в L3.4 как эксперимент. Для рабочих/нормальных проектов — НЕ рекомендуется, лучше вынести каждый сервис в свой репо или хотя бы извлечь общие контракты в отдельный публичный модуль.

```
<root>/                    # ТОЛЬКО когда реально нужно (исключение, не дефолт)
├── go.work
├── docker/postgres/docker-compose.yml
├── docker/kafka/docker-compose.yaml
├── services/contracts/    # dto, model, storage interfaces — никакой логики
├── services/<api>/
├── services/<worker>/
└── .golangci.yml
```

Скаффолд новой службы — скрипт `mkservice.sh <service> [module_path]` из корня репозитория (он создаёт `services/<svc>` со всеми поддиректориями и дефолтным `main.go`).

---

## 3. Конфиг (cleanenv)

`internal/config/config.go`:

```go
// Package config содержит структуры конфигурации и загрузчик конфигурации.
package config

import (
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env string `yaml:"env" env-default:"local"`

	Storage    Storage    `yaml:"storage"`
	HTTPServer HTTPServer `yaml:"http_server"`
}

type Storage struct {
	Host            string        `yaml:"host" env-required:"true"`
	Port            int           `yaml:"port" env-default:"5432"`
	User            string        `yaml:"user" env-required:"true"`
	Password        string        `yaml:"password" env-required:"true"`
	DBName          string        `yaml:"dbname" env-required:"true"`
	SSLMode         string        `yaml:"sslmode" env-default:"disable"`
	MaxOpenConns    int           `yaml:"max_open_conns" env-default:"10"`
	MaxIdleConns    int           `yaml:"max_idle_conns" env-default:"5"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env-default:"30m"`
}

type HTTPServer struct {
	Address     string        `yaml:"address" env-default:"localhost:8080"`
	Timeout     time.Duration `yaml:"timeout" env-default:"4s"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s"`
}

// MustLoad — fallback на CONFIG_PATH, log.Fatal при любой ошибке.
func MustLoad(configPath string) *Config {
	if configPath == "" {
		configPath = os.Getenv("CONFIG_PATH")
	}
	if configPath == "" {
		log.Fatal("config path not provided: pass argument or set CONFIG_PATH")
	}
	if _, err := os.Stat(configPath); err != nil {
		log.Fatalf("config file does not exist: %s", configPath)
	}
	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("cannot read config: %s", err)
	}
	return &cfg
}
```

Правила:
- Тэги: `yaml:"…"` + `env-default:"…"` или `env-required:"true"`. Никаких 3-в-1 слотов.
- Каждая логическая группа (Storage / HTTPServer / Kafka / Redis / Storage{Original,Processed}Dir / UploadConf) — отдельный struct.
- GoDoc-комментарии на русском допустимы и желательны для Config-структур.
- `MustLoad` — единственный способ загрузить конфиг. Никаких `viper`, никаких ad-hoc парсеров.
- `local.yaml` лежит в `config/local.yaml`. По умолчанию main грузит именно этот путь, переопределить — через ENV `CONFIG_PATH`.

`config/local.yaml`:

```yaml
env: "local"

storage:
  host: localhost
  port: 5433              # часто нестандартные порты, чтобы не конфликтовать
  user: postgres
  password: postgres
  dbname: postgres

http_server:
  address: localhost:8080
  timeout: 4s
  idle_timeout: 60s
```

---

## 4. main.go — каноническая последовательность

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"

	"<service>/internal/config"
	"<service>/internal/handlers"
	customLog "<service>/internal/middleware/logger"
	"<service>/internal/repo/postgres"
)

const (
	localEnv = "local"
	prodEnv  = "prod"
	devEnv   = "dev"
)

func main() {
	// 1) config + logger
	cfg := config.MustLoad("config/local.yaml")
	log := setupLogger(cfg.Env)

	// 2) DSN + sql.Open + Ping (с context-таймаутом 3s)
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.User,
		cfg.Storage.Password, cfg.Storage.DBName, cfg.Storage.SSLMode,
	)
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Error("failed to open postgres", slog.Any("err", err))
		os.Exit(1)
	}
	sqlDB.SetMaxOpenConns(cfg.Storage.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Storage.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Storage.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		log.Error("failed to ping postgres", slog.Any("err", err))
		os.Exit(1)
	}

	// 3) wiring зависимостей: repo → service → handler
	pgRepo := postgres.New(sqlDB)
	hl := handlers.New(pgRepo, pgRepo /*, ...*/)

	// 4) chi router + middleware (порядок важен)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)         // если за прокси
	r.Use(customLog.New(log))        // наш slog access-logger
	r.Use(middleware.Recoverer)

	// 5) routes
	r.Get("/items", hl.GetItems)
	r.Post("/items", hl.CreateItem)
	// …

	// 6) static UI (по желанию)
	fs := http.FileServer(http.Dir("./web"))
	r.Get("/*", fs.ServeHTTP)

	// 7) http.Server с таймаутами из конфига
	srv := &http.Server{
		Addr:         cfg.HTTPServer.Address,
		Handler:      r,
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}
	log.Info("server starting", slog.String("addr", cfg.HTTPServer.Address))

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("server stopped with error", slog.Any("err", err))
		os.Exit(1)
	}
}

func setupLogger(env string) *slog.Logger {
	switch env {
	case localEnv:
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	case devEnv:
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	case prodEnv:
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	default:
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
}
```

Правила:
- Порядок чтения: `config → logger → внешние клиенты (db, redis, kafka) → repo → service → handlers → router → server`.
- Любая инициализационная ошибка → `log.Error(...); os.Exit(1)`. Не возвращай ошибки из `main`.
- Errors всегда логгируются с `slog.Any("err", err)` (в новых сервисах L3.3/L3.4) — старый код использует `slog.Any("error", err)`; **новый код пиши с `"err"`**.
- В сервисах с долгоживущими внешними ресурсами (Kafka, БД с явным close, фоновые воркеры) — обязательно graceful shutdown по образцу L3.4:
  ```go
  stop := make(chan os.Signal, 1)
  signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

  go func() {
      log.Info("http server started", slog.String("addr", cfg.Server.Addr))
      if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
          log.Error("server stopped with error", slog.Any("err", err))
          os.Exit(1)
      }
  }()

  <-stop
  log.Info("shutdown signal received")

  ctxSh, cancelSh := context.WithTimeout(context.Background(), 5*time.Second)
  defer cancelSh()
  _ = srv.Shutdown(ctxSh)
  // _ = producer.Close(); _ = consumer.Close(); _ = db.Close()
  ```
  Важно: `srv.ListenAndServe()` блокирующий — он не возвращается до `srv.Shutdown(...)`. Поэтому его обязательно нужно запускать в горутине, иначе блок `<-stop` никогда не сработает (это ошибка в L3.2 main, не повторяй).

---

## 5. Слои и интерфейсы

Архитектура трёхслойная: **handlers → service → repo/cache**. Service — опционален: если CRUD прямой, handler может звать repo напрямую (как в L3.3). Если есть бизнес-логика (генерация alias, кэш-стратегия, fan-out) — обязательно отдельный `service`.

### 5.1. Зависимости handler-а — это **интерфейсы потребителя**

В L3.3 каждое действие вынесено в свой одно-методный "Verb-er" интерфейс в файле своего хендлера:

```go
// internal/handlers/post_comment.go
type CommentCreator interface {
	Create(ctx context.Context, parentID *int64, body string) (dto.Comment, error)
}

// internal/handlers/get_comment.go
type CommentGetter interface {
	GetSubtree(ctx context.Context, rootID int64, maxDepth int) (dto.CommentNode, error)
}
```

И в `handler.go`:

```go
type Handler struct {
	creator     CommentCreator
	getter      CommentGetter
	deleter     CommentDeleter
	rootsGetter RootCommentsGetter
	searcher    CommentSearcher
}

func New(
	creator CommentCreator,
	getter CommentGetter,
	deleter CommentDeleter,
	rootsGetter RootCommentsGetter,
	searcher CommentSearcher,
) *Handler { … }
```

В `main.go` все они получаются из одного `*postgres.PostgresRepo`:
```go
hl := handlers.New(pgRepo, pgRepo, pgRepo, pgRepo, pgRepo)
```

Это намеренно — каждое действие зависит только от своего минимального интерфейса (ISP), но реализован один repo.

Если service-слой есть (как в L3.2 shortener) — handler зависит от **interface** сервиса (`service.Shortener`, `service.Redirector`), не от *Service.

### 5.2. Конструкторы

- Все `New(...)` принимают зависимости **явно через параметры** — это дефолт. Option-pattern (functional options) допустим, но не используется без необходимости: оправдан только когда у конструктора реально много опциональных полей с разумными дефолтами. Если сомневаешься — пиши обычный `New(dep1, dep2, ...)`.
- `New(...)` возвращает **указатель на конкретный struct** (`*ShortenerService`, `*PostgresRepo`, `*Handler`). Исключение: когда нужна полиморфная фабрика (`NewImageProcessor` возвращает `ImageProcessor` interface).
- В конструкторе, если принимается logger — обогащаем его контекстом:
  ```go
  return &ShortenerService{
      …,
      log: log.With(slog.String("service", "shortener")),
  }
  ```
- Защита от `nil` логгера (опциональна, но в L3.4 принята):
  ```go
  if logger == nil { logger = slog.Default() }
  baseLog := logger.With(slog.String("layer", "http"), slog.String("component", "handlers"))
  ```

### 5.3. mockgen + generate-комментарии

Над каждым "выходящим" интерфейсом (`repo.Repo`, `cache.Cache`, прочие) ставим:

```go
//go:generate mockgen -source=cache.go -destination=./mocks/mock_cache.go -package=cachemocks
```

Моки кладём в **`mocks/`** рядом с интерфейсом (НЕ в `internal/mocks`). Названия пакетов моков — `<name>mocks` (`cachemocks`, `repomocks`).

---

## 6. Handlers

### 6.1. Стиль L3.3 — **один эндпоинт = один файл**

Файл `internal/handlers/post_comment.go` содержит:
1. `XxxRequest` / `XxxResponse` структуры с JSON-тегами.
2. Verb-er интерфейс (`CommentCreator`).
3. Метод `(h *Handler) CreateComment(...)`.

```go
type CreateCommentRequest struct {
	ParentID *int64 `json:"parent_id"`  // null для корневого
	Body     string `json:"body"`
}

type CreateCommentResponse struct {
	CreatedComment dto.Comment `json:"created_comment"`
}

type CommentCreator interface {
	Create(ctx context.Context, parentID *int64, body string) (dto.Comment, error)
}

func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
	var req CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		http.Error(w, "body is required", http.StatusBadRequest)
		return
	}

	created, err := h.creator.Create(r.Context(), req.ParentID, req.Body)
	if err != nil {
		log.Printf("%v", err) // или slog
		http.Error(w, "failed to create comment", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(CreateCommentResponse{CreatedComment: created})
}
```

### 6.2. Маппинг ошибок на HTTP-статусы

Используем `errors.Is`/`errors.As` + `switch`:

```go
if err != nil {
	switch {
	case errors.Is(err, service.ErrAliasAlreadyExists):
		log.Warn("alias already exists", slog.Any("err", err))
		http.Error(w, "alias already exists", http.StatusConflict)
	case errors.Is(err, service.ErrFailedToGenerateAlias):
		log.Error("alias generation exhausted", slog.Any("err", err))
		http.Error(w, "failed to generate short url", http.StatusInternalServerError)
	default:
		log.Error("shorten failed", slog.Any("err", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
	return
}
```

### 6.3. Контекст логгера в хендлере

```go
const op = "handlers.shorten"
log := h.log.With(slog.String("op", op))
```

В L3.4 дополнительно вешаем `slog.String("method", "Upload")` и в каждом шаге обогащаем `log = log.With(slog.String("filename", filename))`.

### 6.4. Status code conventions

| Случай | Status |
|---|---|
| POST успешно создал ресурс | `201 Created` |
| POST/PUT возвращает данные | `200 OK` |
| GET успех | `200 OK` |
| Async задача принята | `202 Accepted` |
| DELETE без тела | `204 No Content` |
| DELETE с возвратом ID | `200 OK` + `{deleted_id}` |
| Bad input | `400 Bad Request` |
| Unique violation / уже существует | `409 Conflict` |
| Not found | `404 Not Found` |
| Слишком большой файл | `413 Request Entity Too Large` |
| Внешний сервис недоступен | `502 Bad Gateway` |
| Прочие ошибки | `500 Internal Server Error` |

### 6.5. Тело ошибки — **по возможности plain text** через `http.Error`

```go
http.Error(w, "alias already exists", http.StatusConflict)
```

Дефолт — plain text. JSON-конверт `{"error":"..."}` допустим, если в существующем сервисе так уже сделано или этого требует фронт; не смешивай в рамках одного сервиса. Для успешных JSON-ответов — `writeJSON(w, status, v)` helper:

```go
// внизу handlers.go
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "json encode error", http.StatusInternalServerError)
	}
}
```

### 6.6. Валидация / парсинг query

- `strings.TrimSpace` + проверка пустоты.
- `strconv.Atoi`/`ParseInt` с явными границами: `v <= 0`, `v > 100`, `v < 0`.
- `sort`/`order` — **whitelist через switch**, не интерполяция в SQL:
  ```go
  if s := r.URL.Query().Get("sort"); s != "" {
      s = strings.ToLower(s)
      if s != "created_at" && s != "id" {
          http.Error(w, "invalid sort (created_at|id)", http.StatusBadRequest)
          return
      }
      q.Sort = s
  }
  ```

### 6.7. Пагинация

`limit + 1` трюк для `has_more`:

```go
limitPlus := q.Limit + 1
// SELECT ... LIMIT $1 OFFSET $2 → передаём limitPlus
hasMore := false
if len(items) > q.Limit {
    hasMore = true
    items = items[:q.Limit]
}
```

В ответе всегда есть `items`, `limit`, `offset`, `has_more`.

### 6.8. JSON pretty-print

Для GET-ответов с деревьями/списками — индент 2 пробела:

```go
enc := json.NewEncoder(w)
enc.SetIndent("", "  ")
_ = enc.Encode(resp)
```

---

## 7. Service (бизнес-логика)

`internal/service/<name>.go`:

```go
// Package service содержит бизнес-логику <name>-сервиса.
package service

type Shortener interface {
	Shorten(ctx context.Context, url, alias string) (string, error)
}

type ShortenerService struct {
	repo  repo.Repo
	cache cache.Cache
	log   *slog.Logger
}

func NewShortener(rp repo.Repo, ca cache.Cache, log *slog.Logger) *ShortenerService {
	return &ShortenerService{
		repo:  rp,
		cache: cache,
		log:   log.With(slog.String("service", "shortener")),
	}
}
```

Правила:
- Константы (TTL, лимиты, max attempts) — `const` в файле сервиса или в начале функции (`const aliasLen = 8`).
- Опциональные операции (запись в кэш) логируются `Warn`, но **ошибка не возвращается клиенту**.
- Sentinel ошибки в `errors.go`:
  ```go
  var (
      ErrAliasAlreadyExists    = errors.New("alias already exists")
      ErrFailedToGenerateAlias = errors.New("failed to generate alias")
  )
  ```
- Ошибки repo оборачиваются в собственные ErrXxx, чтобы handler не зависел от внутренней реализации.
- Проверки типа `pq.Error.Code == "23505"` для unique violation:
  ```go
  func isUniqueViolation(err error) bool {
      var pqErr *pq.Error
      if errors.As(err, &pqErr) {
          return pqErr.Code == "23505"
      }
      return false
  }
  ```

---

## 8. Repo (PostgreSQL)

```go
package postgres

type PostgresRepo struct {
	db *sql.DB
}

func New(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

func (s *PostgresRepo) Create(ctx context.Context, parentID *int64, body string) (dto.Comment, error) {
	const query = `
		INSERT INTO comments (parent_id, body)
		VALUES ($1, $2)
		RETURNING id, created_at
	`
	var (
		id        int64
		createdAt sql.NullTime
	)
	if err := s.db.QueryRowContext(ctx, query, parentID, body).Scan(&id, &createdAt); err != nil {
		return dto.Comment{}, err
	}
	c := dto.Comment{ID: id, ParentID: parentID, Body: body}
	if createdAt.Valid {
		c.CreatedAt = createdAt.Time
	}
	return c, nil
}
```

Правила:
- Все SQL — **`const query = \`…\``** в начале функции. Не строй query из format'а с пользовательским вводом.
- Динамический `ORDER BY` — только через whitelist + `fmt.Sprintf("%s %s", sortCol, order)`, где обе переменные предварительно валидированы switch'ем.
- Нullable поля — `sql.NullTime`, `sql.NullString`, `sql.NullInt64`. Конвертируй в `*T` при возврате DTO.
- `sql.ErrNoRows` → `repo.ErrNotFound` (свой sentinel в `internal/repo/repo.go`):
  ```go
  if errors.Is(err, sql.ErrNoRows) {
      return model.Image{}, repo.ErrNotFound
  }
  ```
- `defer rows.Close()` после `QueryContext`. Проверяй `rows.Err()` после цикла.
- Pre-allocate slices: `make([]dto.X, 0, q.Limit+1)`.
- Длинные SQL-запросы пиши **многострочно** с отступами в виде табов внутри `\`…\`` для читаемости.

Миграции (`migrations/0001_init.up.sql`):

```sql
CREATE TABLE IF NOT EXISTS comments (
  id         BIGSERIAL PRIMARY KEY,
  parent_id  BIGINT NULL REFERENCES comments(id) ON DELETE CASCADE,
  body       TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_comments_parent ON comments(parent_id);
```

`down.sql` всегда есть (`DROP TABLE IF EXISTS …`). Каждая миграция — пара up/down.

`cmd/migrate/migrations.go` — стандартный CLI:

```go
action := flag.String("action", "up", "up | down | step")
steps := flag.Int("n", 1, "Количество шагов для step")
flag.Parse()

cfg := config.MustLoad("config/local.yaml")
dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", …)
m, err := migrate.New("file://migrations", dbURL)
// switch *action: m.Up() / m.Down() / m.Steps(*steps)
```

Запуск: `go run ./cmd/migrate -action up`.

---

## 9. Cache (Redis, write-through)

```go
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

//go:generate mockgen -source=cache.go -destination=./mocks/mock_cache.go -package=cachemocks
```

Стратегия — **read-through с touch TTL** + **write-through на запись**:
1. `cache.Get` → если hit, освежаем TTL через `Set` (ошибки игнорируем с `Warn`).
2. Если miss → `repo.Get` → `cache.Set` (write-through, ошибки кэша не валят запрос).

См. `RedirectService.Resolve`.

---

## 10. Logger middleware

`internal/middleware/logger/logger.go` — **ВСЕГДА** свой slog access-logger, не chi default:

```go
func New(log *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		log := log.With(slog.String("component", "middleware/logger"))
		log.Info("logger middleware enabled")
		fn := func(w http.ResponseWriter, r *http.Request) {
			entry := log.With(
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
				slog.String("request_id", middleware.GetReqID(r.Context())),
			)
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			t1 := time.Now()
			defer func() {
				entry.Info("request completed",
					slog.Int("status", ww.Status()),
					slog.Int("bytes", ww.BytesWritten()),
					slog.String("duration", time.Since(t1).String()),
				)
			}()
			next.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}
}
```

Импорт-алиас в main: `customLog "<svc>/internal/middleware/logger"` (или `mwLogeer` / `mwLogger`). Регистрируется ПОСЛЕ `RequestID` и `RealIP`, но ПЕРЕД `Recoverer`.

---

## 11. Тесты

### 11.1. Раскладка

- `*_test.go` рядом с тестируемым кодом, **тот же package** (white-box).
- Моки в `internal/<layer>/mocks/mock_<name>.go`, генерируются `mockgen`.
- В тестах сервисов: `cachemocks "shortener/internal/cache/mocks"`, `repomocks "shortener/internal/repo/mocks"`.

### 11.2. Шаблон unit-теста сервиса

```go
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
}

func TestRedirectService_Resolve_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	rp := repomocks.NewMockRepo(ctrl)
	ca := cachemocks.NewMockCache(ctrl)

	ca.EXPECT().Get(gomock.Any(), "abc123").Return("https://example.com", nil)
	ca.EXPECT().Set(gomock.Any(), "abc123", "https://example.com", cacheTTL).Return(nil)
	rp.EXPECT().GetURL(gomock.Any(), gomock.Any()).Times(0)

	svc := NewRedirect(rp, ca, testLogger())
	got, err := svc.Resolve(ctx, "abc123")
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if got != "https://example.com" {
		t.Fatalf("expected url %q, got %q", "https://example.com", got)
	}
}
```

Стилевые правила:
- Naming: `Test<Type>_<Method>_<Scenario>` через `_`.
- `t.Fatalf("expected X, got %v", got)` — bare ifs, без assertion-обёрток в коротких тестах сервисов.
- `gomock.Any()` для context, конкретные значения для бизнес-аргументов.
- `Times(0)` для проверки что зависимость **не дёрнулась**.

### 11.3. HTTP handlers

В L3.4 handler-тесты — через `httptest`:

```go
func setupHandler(t *testing.T) (*Handler, *repoMocks.MockImageRepo, *storageMocks.MockAPIStorage, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockRepo := repoMocks.NewMockImageRepo(ctrl)
	mockStorage := storageMocks.NewMockAPIStorage(ctrl)
	h := &Handler{Logger: newDefaultLogger(), DB: mockRepo, Storage: mockStorage, MaxUploadBytes: 10 << 20}
	return h, mockRepo, mockStorage, ctrl
}

func TestGetImage_NotFound(t *testing.T) {
	h, mockRepo, _, ctrl := setupHandler(t)
	defer ctrl.Finish()

	mockRepo.EXPECT().Get(gomock.Any(), "unknown-id").Return(model.Image{}, repo.ErrNotFound)

	r := chi.NewRouter()
	r.Get("/image/{id}", h.GetImage)
	req := httptest.NewRequest(http.MethodGet, "/image/unknown-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
```

Здесь — `testify/assert` + `testify/require`, моки через chi-роутер чтобы URL params работали.

### 11.4. Helper-фикстуры

- `t.Helper()` в любом setup-helper'е.
- Генераторы тестовых данных (`createTestJPEG`, `createTestPNG`) — отдельные функции, тоже с `t.Helper()`.

---

## 12. Async / Kafka воркер

См. `L3.4/services/worker`. Цикл:

```
fetch → parse job (commit на bad json) → check repo (skip+commit на NotFound)
      → process → save → MarkReady → commit
```

Идемпотентность:
- На каждом шаге проверяй текущий `Status` в БД (`processing`/`ready`/`failed`).
- Уже `ready`/`failed` → **commit и пропускай**.
- БД временно недоступна → **НЕ commit** (Kafka даст повтор).
- На фатальной ошибке обработки → `MarkFailed(best-effort)` + commit.

`retry.Strategy` для всех внешних retry-able операций:

```go
fetchRetry := retry.Strategy{Attempts: 10, Delay: 1 * time.Second, Backoff: 2}
kafkaRetryStrategy := retry.Strategy{Attempts: 3, Delay: 2 * time.Second, Backoff: 2}
```

Producer / Consumer создаются в main. `producer.Writer.AllowAutoTopicCreation = true` для DX в локалке.

---

## 13. Контракты в монорепо (`services/contracts`) — exception only

Это **не дефолт**. Вынесение в монорепо имеет смысл только для тесно связанных сервисов одного приложения (типа image-processor + worker в L3.4). В обычном случае — каждый сервис в своём репо со своим go.mod.

Когда всё-таки нужно (>1 сервиса с общими DTO в одном репо):

```
services/contracts/
├── dto/image_job.go        # JSON-сообщения (Kafka payloads)
├── model/image.go          # доменные модели + Status enum (string-based)
└── storage/storage.go      # interfaces для разных потребителей (APIStorage, WorkerStorage)
```

Status — `type Status string` + константы `StatusProcessing`/`StatusReady`/`StatusFailed`. В БД — строкой через CHECK constraint:

```sql
status TEXT NOT NULL CHECK (status IN ('processing', 'ready', 'failed'))
```

`go.work` в корне:

```go
go 1.25.3

use (
	./services/contracts
	./services/image-processor
	./services/worker
)
```

---

## 14. Линтер (`.golangci.yml`)

В L3.4 настроен следующий `.golangci.yml` — копируй as-is для новых проектов:

```yaml
run:
  timeout: 5m
  issues-exit-code: 1
  tests: true

linters-settings:
  govet:
    enable-all: true
    disable:
      - fieldalignment
      - shadow
  gci:
    sections:
      - standard
      - default
      - prefix(contracts)
      - prefix(<service-1>)
      - prefix(<service-2>)
  revive:
    severity: warning
  errcheck:
    check-type-assertions: false
    check-blank: false

linters:
  disable-all: true
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - typecheck
    - unused
    - gci
    - goimports
    - revive
    - errorlint
    - misspell
    - unconvert
    - bodyclose
    - copyloopvar
    - durationcheck
    - nilerr
    - makezero
    - prealloc
    - predeclared

issues:
  exclude-use-default: false
  max-issues-per-linter: 0
  max-same-issues: 0
```

Импорты группируй gci-стилем: stdlib → 3rd-party → `prefix(contracts)` → `prefix(<свой-модуль>)`, между группами пустая строка.

---

## 15. Стилевые мелочи

- **Комментарии**: GoDoc на русском (как в исходниках) — нормально и желательно для пакетных и публичных типов. Внутри функций — короткие нумерованные шаги «// 1) парсим …», «// 2) валидируем …». Англо-русский микс — допустим.
- **Doc-комментарий пакета** есть везде, где есть экспортируемые типы: `// Package config содержит …`.
- **Пустая строка после early-return блоков** — для читаемости.
- **`_ = json.NewEncoder(w).Encode(...)`** — приёмок, не warning. Encode-ошибки в success-пути молча игнорируем (после WriteHeader всё равно ничего не сделать).
- **Defer для close**: `defer rows.Close()`, `defer file.Close()`, `defer func() { _ = rc.Close() }()`.
- **Helpers** в конце файла под комментарием `// Helpers`.
- **`TODO:` комментарии** допустимы для будущих доработок (например, `// TODO: prod, dev ?` в setupLogger).
- **Нет глобальных переменных кроме sentinel-ошибок** и `Validate = validator.New()`.

---

## 16. Docker compose

Каждый сервис тянет свой постгрес/редис из своего `docker/<svc>/docker-compose.yml`:

```yaml
services:
  postgres:
    image: postgres:17
    container_name: postgres-<svc>
    environment:
      POSTGRES_DB: postgres
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - "5433:5432"        # 5433/5434/… чтобы не конфликтовать
    volumes:
      - pgdata_<svc>:/var/lib/postgresql/data

volumes:
  pgdata_<svc>:
```

Kafka (L3.4 пример) — в `docker/kafka/docker-compose.yaml` рядом.

`postgres/postgres/postgres` как локальные creds — норм (это не настоящий секрет).

---

## 17. README на сервис

Минимум (L3.3 шаблон):

````markdown
# <service>

Краткое описание: что делает сервис, основные эндпоинты.

## API
- POST /…
- GET /…
- DELETE /…

## Запуск

### 1) PostgreSQL в Docker
```bash
cd docker/postgres
docker compose up -d
```

### 2) Миграции
```bash
cd services/<service>
go run ./cmd/migrate -action up
```

### 3) Сервис
```bash
cd services/<service>
go run ./cmd/<service>
```

## Конфиг
`config/local.yaml`:
```yaml
…
```
````

---

## 18. Чеклист «Я добавил новый сервис»

- [ ] Создан скаффолд (`./mkservice.sh <svc> <module-path>` или вручную, см. §2).
- [ ] `go.mod` инициализирован, Go 1.25.x.
- [ ] `internal/config/config.go` + `config/local.yaml` (env, storage, http_server + спец-секции при необходимости).
- [ ] `internal/middleware/logger/logger.go` (slog access-logger).
- [ ] `internal/repo/repo.go` (interface) + `internal/repo/postgres/postgres.go` (impl) + sentinel `repo.ErrNotFound`.
- [ ] `internal/handlers/handler.go` (struct + `New(...)`) + по файлу на эндпоинт.
- [ ] DTO в `internal/dto/` или `internal/handlers/dto/`.
- [ ] Если есть бизнес-логика — `internal/service/<name>.go` + `errors.go`.
- [ ] Если есть кэш — `internal/cache/cache.go` + `internal/cache/redis/redis.go`.
- [ ] `migrations/0001_init.up.sql` + `0001_init.down.sql`.
- [ ] `cmd/<svc>/main.go` следует §4. Graceful shutdown — если есть Kafka/долгоживущие клиенты.
- [ ] `cmd/migrate/migrations.go` (CLI с `-action`).
- [ ] `web/index.html` + `web/app.js` если нужен тестовый UI (необязательно).
- [ ] `docker/<svc>/docker-compose.yml` для локальной БД.
- [ ] `README.md` по шаблону §17.
- [ ] `//go:generate mockgen` над interface'ами + сгенерированные моки в `mocks/`.
- [ ] Тесты сервиса (gomock + bare ifs) и/или хендлеров (httptest + testify).
- [ ] `.golangci.yml` (см. §14) если новый репозиторий.
- [ ] `go mod tidy`, `go vet ./...`, `golangci-lint run ./...`, `go test ./...` — всё зелёное.

---

## 19. Anti-patterns (не делай так)

- ❌ `viper`, ручной `flag`-парсинг конфига, гигантская `.env`. Только `cleanenv` + YAML.
- ❌ Передача `*sql.DB` напрямую в handler. Только через repo interface.
- ❌ Глобальный `*slog.Logger`. Только через DI в конструкторах.
- ❌ Один файл `handlers.go` с десятком хендлеров — для L3.3-стиля разбивай по эндпоинту.
- ❌ JSON-конверт ошибок (`{"error": "..."}`). Используй `http.Error` (plain text).
- ❌ Динамический `ORDER BY $1` — Postgres так не умеет, и это unsafe. Whitelist + `fmt.Sprintf` после валидации.
- ❌ Игнорирование `sql.ErrNoRows`. Всегда мапить в `repo.ErrNotFound`.
- ❌ `panic` в продакшен-коде. `log.Error + os.Exit(1)` в `main`, ошибки наверх — обычным `error`.
- ❌ `interface{}` / `any` в публичных API без необходимости. Конкретные типы.
- ❌ Тесты, изменяющие глобальное состояние или зависящие от внешней БД — для unit'ов используем gomock.
- ❌ Имена типа `IService`, `Impl`. Используем `Service` / `*ShortenerService`.
- ❌ Многомодульный монорепо ради двух сервисов «потому что прикольно». Разделяй на репозитории, общий код — отдельный модуль/библиотека.
- ❌ `srv.ListenAndServe()` без горутины + ожидание сигнала после него (как в L3.2 main) — `<-stop` никогда не сработает, потому что `ListenAndServe` блокирующий. Запускай его в `go func() { ... }()`.

---

## 20. Файлы-эталоны (открой их и подражай)

- main: `L3.3/services/commenttree/cmd/commenttree/main.go`
- config: `L3.3/services/commenttree/internal/config/config.go`
- handler structure + per-verb files: `L3.3/services/commenttree/internal/handlers/`
- middleware logger: `L3.3/services/commenttree/internal/middleware/logger/logger.go`
- service слой + кэш: `L3.2/services/shortener/internal/service/`
- service unit-тесты с gomock: `L3.2/services/shortener/internal/service/redirect_service_test.go`
- handler-тесты с httptest+testify: `L3.4/services/image-processor/internal/handlers/handlers_test.go`
- migrate CLI: `L3.3/services/commenttree/cmd/migrate/migrations.go`
- Kafka worker (idempotent loop): `L3.4/services/worker/internal/kafka_consumer/worker_consumer.go`
- contracts (multi-svc): `L3.4/services/contracts/`
- mkservice scaffold script: `mkservice.sh` в корне репо

Если сомневаешься в конкретном решении — сначала смотри в L3.3, потом в L3.4 (как более новый), потом в L3.2.
