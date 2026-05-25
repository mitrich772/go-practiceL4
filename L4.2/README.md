# mygrep — распределённый `grep` с кворумом

Клиент режет вход на чанки, шлёт их 3 серверам по HTTP и собирает результат,
когда ответили `≥ N/2+1` узлов (кворум).
Внутри каждого сервера — пул горутин-воркеров с обменом через каналы.

---

## Что нужно

- Docker + Docker Compose (для серверов)
- Go 1.22+ (для сборки клиента)

## Запуск — 3 шага

```bash
# 1. поднять 3 сервера в docker (ждёт healthy всех трёх)
make up

# 2. собрать клиент
make build

# 3. искать
./bin/mygrep --servers 127.0.0.1:9101,127.0.0.1:9102,127.0.0.1:9103 \
             -F -n -e ERROR examples/data/access.log
```

Ожидаемый результат:
```
4:2025-05-25T10:00:04Z ERROR service=api request=/orders  status=500 err="db timeout"
8:2025-05-25T10:00:08Z ERROR service=worker job=email err="smtp unreachable"
12:2025-05-25T10:00:12Z ERROR service=api request=/orders  status=500 err="db timeout"
18:2025-05-25T10:00:18Z ERROR service=api request=/payments status=502 err="upstream"
```

Погасить кластер:
```bash
make down
```

---

## Сравнение с системным `grep`

```bash
make compare
```

Скрипт сам поднимает 3 сервера, прогоняет 11 кейсов на разных флагах,
сверяет вывод побайтно с системным `grep` и в конце гасит сервера. Итог:

```
== Сравнение mygrep vs системного grep ==
  [PASS] fixed ERROR (default)             exit=(grep=0 mygrep=0)
  [PASS] fixed ERROR -n                    exit=(grep=0 mygrep=0)
  [PASS] fixed ERROR -c                    exit=(grep=0 mygrep=0)
  [PASS] fixed status=200 -v               exit=(grep=0 mygrep=0)
  [PASS] regex ^2025-05-25T10:00:0[1-3]    exit=(grep=0 mygrep=0)
  [PASS] regex status=(500|502) -n         exit=(grep=0 mygrep=0)
  [PASS] ignore case 'error' -i            exit=(grep=0 mygrep=0)
  [PASS] no matches (DOESNOTEXIST)         exit=(grep=1 mygrep=1)
  [PASS] words fixed 'lima' -i             exit=(grep=0 mygrep=0)
  [PASS] words regex ^[A-Z]                exit=(grep=0 mygrep=0)
  [PASS] words count letters -c            exit=(grep=0 mygrep=0)
== Итог: PASS=11  FAIL=0 ==
```

---

## Unit-тесты

```bash
make test       # go test -race ./...
```

Покрытие: разбиение на чанки, matcher, HTTP-обработчик, клиентский кворум,
ретраи при недоступном сервере, разбор групп коротких флагов.

---

## Флаги `mygrep`

```
mygrep --servers host1:port,host2:port,... -e PATTERN [flags] [file]
```

| Флаг           | Аналог `grep` | Что делает                                       |
|----------------|---------------|--------------------------------------------------|
| `-e PATTERN`   | `-e`          | Регулярка (RE2)                                  |
| `-F`           | `-F`          | Фиксированная строка (без regex)                 |
| `-i`           | `-i`          | Игнорировать регистр                             |
| `-v`           | `-v`          | Инвертировать совпадение                         |
| `-n`           | `-n`          | Печатать номера строк (глобальные)               |
| `-c`           | `-c`          | Только количество совпадений                     |
| `-l`           | `-l`          | Только имя файла, если есть совпадения           |
| `--servers`    | —             | Список адресов узлов через запятую (обязательно) |
| `--quorum N`   | —             | Кворум (по умолчанию `floor(N/2)+1`)             |
| `--timeout S`  | —             | Таймаут одного HTTP-запроса (сек)                |

Короткие флаги можно объединять: `-Fni ERROR file.log` ≡ `-F -n -i ERROR file.log`.

Код возврата как у `grep`: `0` — есть совпадения, `1` — нет, `2` — ошибка.

---

## Что под капотом

```
            stdin / файл
                │
                ▼
          [ mygrep ]                 ← клиент: режет на N чанков
                │
   ┌────────────┼────────────┐       HTTP POST /process
   ▼            ▼            ▼
[server1]   [server2]   [server3]    ← docker compose, каждый: пул горутин
   │            │            │
   └────────────┴────────────┘       результаты в канал
                │
                ▼
       ждём кворум N/2+1
       сортируем по chunk_id
                │
                ▼
             stdout
```

- **Каналы и горутины внутри сервера**: пул воркеров (`runtime.NumCPU()`)
  получает строки из `jobs` канала, пишет совпадения в `out` канал.
- **Каналы между серверами**: клиент стартует горутину на каждый сервер,
  ответы складывает в общий канал, ждёт кворум.
- **Кворум**: считаем достоверным, когда `≥ N/2+1` серверов вернули
  результат. Если меньше — печатаем что есть и warning в stderr.

---

## Полезные `make`-цели

| Команда       | Что делает                                       |
|---------------|--------------------------------------------------|
| `make up`     | docker compose поднимает 3 сервера (с healthy)   |
| `make down`   | гасит и удаляет контейнеры                       |
| `make ps`     | состояние контейнеров                            |
| `make logs`   | логи серверов в реальном времени                 |
| `make build`  | собирает `./bin/mygrep`                          |
| `make test`   | `go test -race ./...`                            |
| `make compare`| 11 кейсов сравнения с системным `grep`           |
| `make clean`  | удаляет `./bin/`                                 |

---

## Структура

```
.
├── cmd/
│   ├── mygrep/             # клиент
│   └── mygrep-server/      # сервер
├── internal/
│   ├── protocol/           # типы запросов/ответов
│   ├── chunk/              # разбиение входа
│   ├── grep/               # matching
│   ├── server/             # HTTP-обработчик + воркер-пул
│   └── client/             # раздача чанков, кворум, ретраи
├── examples/data/
│   ├── access.log          # пример access-лога
│   └── words.txt           # пример словаря
├── scripts/
│   └── compare_with_grep.sh
├── Dockerfile              # образ mygrep-server
├── docker-compose.yml      # 3 ноды на портах 9101/9102/9103
└── Makefile
```
