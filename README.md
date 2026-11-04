# TripGo — лабораторная работа 2

Сервис создания, получения и завершения поездок. Вторая работа сохраняет HTTP API
первой и добавляет разделение слоёв, тесты, линтер и OpenTelemetry.
Контракты и миграции первой работы не изменялись. Бонусное задание не выполнялось.

## Запуск

Нужны Go версии из `go.mod`, Make и установленный `tripgoctl`.

```bash
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
make migrate
make run
```

`environment.toml` выбирает лабораторную 2 с PostgreSQL и observability.
Адреса БД, коллектора, Jaeger, Prometheus и Grafana берутся из `tripgoctl connect`.
Используйте сгенерированный `.env`; `.env.example` показывает остальные параметры.
Не заменяйте свои адреса и секреты примерами из репозитория.
Порты примера соответствуют проверенному локальному окружению и могут отличаться.

Если `.env` остался от первой работы, обновите `DATABASE_URL` и добавьте
`OTEL_EXPORTER_OTLP_ENDPOINT` и `OTEL_EXPORTER_OTLP_PROTOCOL` по данным `connect`.
Для gRPC endpoint записывается как `http://localhost:<порт gRPC>`.
Переменные процесса имеют приоритет над `.env` при запуске Go-приложения.

По умолчанию HTTP слушает `:8080`. PostgreSQL проверяется перед запуском сервера.
Недоступность настроенного коллектора не мешает обслуживать запросы.
Ctrl+C и SIGTERM запускают graceful shutdown в пределах `SHUTDOWN_TIMEOUT`.

## Команды

| Команда | Действие |
|---|---|
| `make build` | Сборка всех пакетов |
| `make run` | Запуск сервиса |
| `make generate` | Генерация OpenAPI и моков |
| `make test` | Unit-тесты с `-race`, двумя запусками и отчётом покрытия |
| `make test-integration` | Тесты с тегом `integration`, реальной БД, `-race` и двумя запусками |
| `make lint` | Обязательные линтеры и ограничения зависимостей |
| `make migrate` | Применение существующих миграций |
| `make migrate-status` | Состояние миграций |
| `make migrate-down` | Откат одной последней миграции с удалением её таблицы |
| `make docker-run` | Сборка и запуск контейнера на порту 8001 |

Makefile читает `.env` как набор присваиваний Make. Для значений со специальными
символами Make используйте подходящее экранирование или аргументы команды.
`coverage.out`, `.env` и `.tripgo/` не коммитятся.

## Архитектура

```text
cmd/trip-service        конфигурация, запуск, ожидание остановки
internal/container      создание и связывание зависимостей
internal/domain/trip    поездка, валидация, переход статуса, ошибки
internal/model          типы обмена между слоями
internal/usecase/trip    create, get, finish
internal/http/trip       create, get, finish и преобразование моделей
internal/http/domainerror преобразование ошибки в Problem
internal/http/middleware  recover, tracing, logging, metrics
internal/repository/trip  параметризованные SQL-запросы
internal/infra/postgres   пул и инструментирование pgx
internal/infra/postgres/transaction менеджер транзакций
internal/infra/telemetry  OTLP, логгер и бизнес-метрики
internal/generated      OpenAPI-код
```

Путь запроса: HTTP → usecase → доменная модель и интерфейсы → репозиторий → БД.
Домен и usecase не импортируют HTTP, pgx, squirrel или сгенерированный API.
API-модели преобразуются только в транспорте. `model` переиспользует доменные
типы через алиасы, чтобы не дублировать одинаковые структуры.

Интерфейсы объявлены в `contract.go` у потребителя и не экспортируются.
Usecase получает только необходимые методы репозитория и менеджера транзакций.
Хендлер получает интерфейс своего usecase. Моки генерируются рядом с тестами
через `go:generate`, отдельного пакета mocks нет.

Конкретные зависимости создаёт только `container.New`. В `main` остаются конфиг,
вызов контейнера, запуск HTTP и ожидание ошибки или сигнала. Существующий путь
`cmd/trip-service` сохранён, чтобы команды запуска и Dockerfile не менялись.
Логгер передаётся зависимостью; глобального бизнес-логгера и глобального пула нет.
Правила зависимостей проверяет `depguard` в `.golangci.yml`.

Переход `active → completed` описывает доменный метод `Trip.Finish`.
Репозиторий сохраняет его условным `UPDATE ... WHERE status = 'active'`:
два одновременных завершения дают один успех и один конфликт.
Частичный уникальный индекс запрещает две активные поездки одному водителю.

Создание и завершение записывают историю в том же `TxManager.Do`.
Транзакция хранится в контексте; вложенный `Do` использует её повторно.
Ошибка или паника вызывают откат. Вне `Do` репозиторий использует пул.
Изоляция `Read Committed` достаточна, поскольку конкурентные изменения защищены
индексом и условным UPDATE, а не проверкой состояния только в Go.

## Конфигурация

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `DATABASE_URL` | Обязательна | PostgreSQL |
| `HTTP_ADDR` | `:8080` | HTTP-сервер |
| `LOG_LEVEL` | `info` | Уровень логов |
| `SHUTDOWN_TIMEOUT` | `30s` | Общий бюджет остановки |
| `DATABASE_MAX_CONNS` / `DATABASE_MIN_CONNS` | `25` / `5` | Размер пула |
| `DATABASE_CONNECT_TIMEOUT` | `5s` | Подключение и стартовый Ping |
| `DATABASE_QUERY_TIMEOUT` | `3s` | Операция БД, readiness и cleanup транзакции |
| `DATABASE_MAX_CONN_LIFETIME` | `30m` | Срок жизни соединения |
| `HTTP_READ_TIMEOUT` | `10s` | Чтение запроса |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | Чтение заголовков |
| `HTTP_WRITE_TIMEOUT` | `15s` | Запись ответа |
| `HTTP_IDLE_TIMEOUT` | `60s` | Keep-alive |
| `OTEL_SERVICE_NAME` | `trip-service` | Имя сервиса |
| `SERVICE_VERSION` | `local` | Версия сборки в ресурсе OTel |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Обязательна | URL коллектора |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` | `grpc` или `http/protobuf` |
| `OTEL_TRACES_SAMPLER` | `parentbased_always_on` | Стратегия сэмплирования |
| `OTEL_TRACES_SAMPLER_ARG` | `1` | Доля для `traceidratio` / `parentbased_traceidratio` |
| `OTEL_METRIC_EXPORT_INTERVAL` | `5000` | Период экспорта в миллисекундах |
| `OTEL_RESOURCE_ATTRIBUTES` | Стандартные env OTel | Дополнительные атрибуты ресурса |

По умолчанию ресурс содержит `service.namespace=tripgo` и
`deployment.environment=local`. Самплеры `always_on`, `always_off` и их
`parentbased`-варианты также поддерживаются. Длительности должны быть
положительными, размер пула — `0 <= min <= max`, `max > 0`.

## Телеметрия

Данные идут через OTLP в коллектор, затем в Jaeger и Prometheus/Grafana.
Порядок middleware: Recover → Tracing → Logging → Metrics → роутер.
HTTP-спаны называются по методу и шаблону маршрута. В атрибутах и метриках
используется `/api/v1/trips/{tripId}`, а не конкретный UUID.
W3C TraceContext принимается из HTTP-заголовков.

SQL инструментируется на уровне пула через `otelpgx`. Обёртка tracer добавляет
имя таблицы и DB-гистограмму с нужными атрибутами. Репозиторий не знает про OTel.
Бизнес-счётчики увеличиваются только после успешного Commit, активные поездки
измеряются по БД при сборе gauge.

Метрики: `http.server.request.duration`, `http.server.active_requests`,
`db.client.operation.duration`, `tripgo.trips.created`, `tripgo.trips.completed`,
`tripgo.trips.active`. Prometheus преобразует точки в подчёркивания и добавляет
суффиксы единиц измерения, histogram и counter.

Логи — JSON с `service`, `trace_id` и `span_id`. На запрос пишется одна access-запись:
2xx/4xx — Info, 5xx — Error. Бизнес-события пишутся отдельно после успешной операции.
Тела и секреты не логируются. Recover возвращает безопасный `500 internal_error`
и записывает панику со стеком.

Готовый дашборд: `deploy/grafana/tripgo.json`. Его можно импортировать в Grafana
через Dashboards → New → Import. Используется datasource `prometheus` окружения курса.
Адреса интерфейсов и учебные реквизиты печатает `tripgoctl connect`.

Результаты проверки: [docs/verification.md](docs/verification.md).

![Трейс HTTP и PostgreSQL](docs/images/jaeger.png)

![Метрики TripGo](docs/images/grafana.png)

## API

POST `/api/v1/trips` создаёт поездку и возвращает 201 и Location.
GET `/api/v1/trips/{tripId}` возвращает поездку, POST на `/finish` завершает её.
Ошибки 400/404/409/500 имеют формат `application/problem+json`.
`/health` отвечает без БД, `/ready` проверяет Ping и отдаёт 200 или 503.

```bash
curl -i localhost:8080/ready
curl -i -X POST localhost:8080/api/v1/trips \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c","driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5","start_point":{"latitude":59.9398,"longitude":30.3146},"end_point":{"latitude":59.929,"longitude":30.3626},"price":1450}'
```

## Docker

Сохранена многостадийная сборка и запуск не от root. Для Docker Desktop укажите
в `.env` `DOCKER_DATABASE_URL` и `DOCKER_OTEL_EXPORTER_OTLP_ENDPOINT` с хостом
`host.docker.internal` и портами БД/коллектора из `connect`, затем `make docker-run`.
Локальный Go-процесс использует `DATABASE_URL` и `OTEL_EXPORTER_OTLP_ENDPOINT`
с адресами, доступными компьютеру. Секреты не копируются в образ.
Проверка контейнера: `curl -i localhost:8001/ready`.
