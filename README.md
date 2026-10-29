# TripGo — лабораторная работа 1

HTTP-сервис создания, получения и завершения поездок с PostgreSQL.
Задание и критерии — в [репозитории курса](https://github.com/course-go-autumn-2026/course).

## Запуск

Нужны Go 1.27.1 (см. `go.mod`), GNU Make, Docker и
[tripgoctl](https://github.com/course-go-autumn-2026/course-infra).
Из корня репозитория:

```bash
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
```

Используйте `.env`, созданный командой `tripgoctl environment start`:
`DATABASE_URL` должен указывать на PostgreSQL, доступный с вашего компьютера.
Не заменяйте его адрес и пароль значениями из `.env.example`.
Переменные окружения имеют приоритет над `.env` в Go-приложении.

Подготовьте зависимости и примените миграции:

```bash
go mod download
make migrate
```

Затем выберите один способ запуска. Файл `.env` менять не нужно.

Локально:

```bash
make run
```

Сервис слушает `HTTP_ADDR` (по умолчанию `:8080`).

В Docker Desktop:

```bash
DOCKER_DATABASE_URL="<адрес PostgreSQL, доступный из контейнера>" make docker-run
```

Сервис доступен на `http://localhost:8001`. Команда собирает образ и передаёт
`DOCKER_DATABASE_URL` как адрес БД контейнеру; файл `.env` остаётся прежним.
Подробнее о настройках Docker — ниже. Остановка — Ctrl+C или SIGTERM.

## Конфигурация

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `DATABASE_URL` | Нет, обязательна | Строка подключения PostgreSQL |
| `HTTP_ADDR` | `:8080` | Адрес HTTP-сервера |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `SHUTDOWN_TIMEOUT` | `30s` | Общий бюджет остановки HTTP и пула |
| `DATABASE_MAX_CONNS` | `25` | Максимум соединений |
| `DATABASE_MIN_CONNS` | `5` | Минимум соединений |
| `DATABASE_CONNECT_TIMEOUT` | `5s` | Подключение и стартовый Ping |
| `DATABASE_QUERY_TIMEOUT` | `3s` | Бюджет операции с БД в HTTP-обработчике |
| `DATABASE_MAX_CONN_LIFETIME` | `30m` | Максимальный срок жизни соединения |
| `HTTP_READ_TIMEOUT` | `10s` | Чтение запроса |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | Чтение заголовков |
| `HTTP_WRITE_TIMEOUT` | `15s` | Запись ответа |
| `HTTP_IDLE_TIMEOUT` | `60s` | Ожидание следующего запроса keep-alive |

Длительности положительные; `0 <= DATABASE_MIN_CONNS <= DATABASE_MAX_CONNS`,
максимум больше нуля. Неверная конфигурация или недоступная при старте БД
завершают запуск с ошибкой. `.env` и `.tripgo/` не коммитятся.
Makefile читает `.env` как присваивания Make: значения с `$` и `#` передавайте
через окружение с подходящим экранированием.

## API

| Метод | Путь | Успешный ответ |
|---|---|---|
| POST | `/api/v1/trips` | `201`, поездка и Location |
| GET | `/api/v1/trips/{tripId}` | `200`, поездка |
| POST | `/api/v1/trips/{tripId}/finish` | `200`, завершённая поездка |
| GET | `/health` | `200`, без обращения к БД |
| GET | `/ready` | `200` при доступной БД, иначе `503` |

Для стандартного `HTTP_ADDR=:8080`:

```bash
curl -i localhost:8080/health
curl -i localhost:8080/ready
curl -i -X POST localhost:8080/api/v1/trips \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c","driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5","start_point":{"latitude":59.9398,"longitude":30.3146},"end_point":{"latitude":59.929,"longitude":30.3626},"price":1450}'
```

Подставьте UUID из ответа вместо `<id>`:

```bash
curl -i 'localhost:8080/api/v1/trips/<id>'
curl -i -X POST 'localhost:8080/api/v1/trips/<id>/finish'
```

Ошибки поездок — `application/problem+json`: `400 invalid_request`,
`404 trip_not_found`, `409 driver_busy` / `trip_completed`, `500 internal_error`.
Детали БД остаются в логах; логи — JSON в stdout после настройки логгера.

## Проверки и команды

```bash
make build
make test
```

Автоматические тесты в первой лабораторной не добавлены.
`make test` оставлен для следующих работ. Цель `make lint` пока отсутствует.

`make generate` обновляет OpenAPI-код; результат коммитится и вручную не правится.
`make migrate-status` показывает миграции; `make migrate-down` откатывает последнюю
миграцию с удалением её таблицы и данных. `make psql` открывает клиент PostgreSQL.
Перед сдачей отдельно проверьте остановку с активными запросами через SIGTERM.

## Решения и ограничения

- `pgxpool` и `squirrel` с параметрами SQL; создание/завершение и запись истории
  выполняются в одной транзакции `Read Committed` через `TxManager`.
- Частичный уникальный индекс запрещает две активные поездки у водителя;
  условный UPDATE защищает от повторного завершения.
- При SIGINT/SIGTERM сначала завершаются HTTP-запросы, затем закрывается пул БД;
  общий срок ограничен `SHUTDOWN_TIMEOUT`.
- Контракты в `contracts/` сохранены. Бонусная идемпотентность не реализована;
  worker pool, gRPC, Kafka, метрики и трейсы относятся к следующим работам.

## Docker

После запуска окружения и применения миграций:

```bash
DOCKER_DATABASE_URL="<адрес PostgreSQL, доступный из контейнера>" make docker-run
curl -i localhost:8001/ready
```

`make docker-run` собирает образ `tripgo` и запускает сервис на порту `8001`.
Передайте в `DOCKER_DATABASE_URL` строку подключения из сгенерированного `.env`,
заменив хост на доступный из контейнера. Для Docker Desktop вместо `localhost`
используйте `host.docker.internal`; порт, пользователя, пароль и имя БД сохраните.
В Docker Engine на Linux укажите адрес, доступный из сети контейнера.
`.env` не меняется: `make migrate` и `make run` используют исходный адрес.
Только сборка образа: `make docker-build`.
Ожидаемый ответ `/ready` — `200`, `{"status":"ok"}`.

Сборка многостадийная, итоговый образ — `distroless/static-debian12:nonroot`,
UID `65532`; `.env` в образ не копируется. Размер и пользователя можно проверить:

```bash
docker image inspect tripgo --format '{{.Size}} bytes, user={{.Config.User}}'
```

Работа сдаётся через PR `homework/01` → `main` своего форка. В описании PR:
что сделано, решения и причины, невыполненные пункты. Слияние — после принятия.
