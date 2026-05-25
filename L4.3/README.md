# L4.3 Calendar Service

HTTP-сервис календаря событий с PostgreSQL-хранилищем, фоновыми воркерами, напоминаниями через канал и асинхронным логгером.

## Что реализовано

- `POST /create_event` - создать событие.
- `POST /update_event?id=1` - обновить событие.
- `POST /delete_event?id=1` - удалить событие.
- `GET /events_for_day?user_id=1&date=2026-05-25` - события за день.
- `GET /events_for_week?user_id=1&date=2026-05-25` - события за ISO-неделю.
- `GET /events_for_month?user_id=1&date=2026-05-25` - события за месяц.
- PostgreSQL хранит активные и архивные события.
- Миграции БД запускаются через CLI-утилиту `cmd/migrate`.
- Reminder worker получает задачи через канал и пишет напоминания в async logger.
- Archive worker каждые `archive_every` переносит старые события в архив через `archived_at`.
- HTTP-хендлеры не пишут в stdout напрямую: они кладут записи в канал логгера.


## Запуск

Команды нужно выполнять из папки `L4.3`, потому что мигратор читает `config/local.yaml` и SQL-файлы из `migrations`.

Поднять PostgreSQL:

```powershell
docker compose up -d
```

Применить все миграции:

```powershell
go run ./cmd/migrate -action up
```

Запустить сервис:

```powershell
go run ./cmd/calendar
```

По умолчанию сервис слушает `localhost:1235`. Настройки лежат в `config/local.yaml`, альтернативный путь можно передать через переменную окружения `CONFIG_PATH`.

## Миграции

Миграции лежат в папке `migrations` и применяются утилитой `cmd/migrate` на базе `golang-migrate`.

Доступные действия:

- `go run ./cmd/migrate -action up` - применить все новые миграции.
- `go run ./cmd/migrate -action down` - откатить все миграции.
- `go run ./cmd/migrate -action step -n 1` - применить одну следующую миграцию.
- `go run ./cmd/migrate -action step -n -1` - откатить одну последнюю миграцию.

Подключение к базе берётся из секции `storage` в конфиге. Для стандартного `docker-compose.yml` используется база `calendar` на `localhost:5434`, пользователь `postgres`, пароль `postgres`.

## Формат события

```json
{
  "user_id": 1,
  "date": "2026-05-25",
  "time": "15:30",
  "name": "meeting",
  "remind_at": "2026-05-25T15:25:00+03:00"
}
```

`remind_at` опционален. Если он указан, время должно быть в будущем.

## Примеры запросов

Для `cmd.exe`.

Создать событие без напоминания:

```cmd
curl.exe -X POST "http://localhost:1235/create_event" -H "Content-Type: application/json" -d "{\"user_id\":1,\"date\":\"2026-12-01\",\"time\":\"15:30\",\"name\":\"meeting\"}"
```

Создать событие с напоминанием:

```cmd
curl.exe -X POST "http://localhost:1235/create_event" -H "Content-Type: application/json" -d "{\"user_id\":1,\"date\":\"2026-12-01\",\"time\":\"15:30\",\"name\":\"meeting\",\"remind_at\":\"2026-12-01T15:25:00+03:00\"}"
```

Обновить событие:

```cmd
curl.exe -X POST "http://localhost:1235/update_event?id=1" -H "Content-Type: application/json" -d "{\"user_id\":1,\"date\":\"2026-12-02\",\"time\":\"10:00\",\"name\":\"updated meeting\"}"
```

Удалить событие:

```cmd
curl.exe -X POST "http://localhost:1235/delete_event?id=1"
```

Получить события за день:

```cmd
curl.exe "http://localhost:1235/events_for_day?user_id=1&date=2026-12-01"
```

Получить события за неделю:

```cmd
curl.exe "http://localhost:1235/events_for_week?user_id=1&date=2026-12-01"
```

Получить события за месяц:

```cmd
curl.exe "http://localhost:1235/events_for_month?user_id=1&date=2026-12-01"
```

## Ответы

Успех:

```json
{"result": "..."}
```

Ошибка:

```json
{"error": "description"}
```

Коды:

- `200 OK` - успешный запрос.
- `400 Bad Request` - ошибка ввода.
- `503 Service Unavailable` - бизнес-ошибка, например событие не найдено.
- `500 Internal Server Error` - непредвиденная ошибка.
