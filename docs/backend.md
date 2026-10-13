# Центральный сервер (`dockrunner_backend`)

Go + Gin. Хранит пользователей и тарифы, выдаёт JWT, владеет RabbitMQ.

## Запуск

```bash
cd dockrunner_backend
cp .env.example .env   # заполнить секреты
go run ./cmd/main
```

Проверки: `GET /health`, `GET /ready` (статус Postgres/InfluxDB).

Живая документация API — Swagger UI: `http://localhost:8080/swagger/` (генерируется из аннотаций командой `swag init -g cmd/main.go -o docs`).

Инфраструктуру для разработки проще поднять контейнерами:
`postgres:16` (5432), `influxdb:2` (8086), `rabbitmq:3-management` (5672).

## Основные переменные окружения

| Переменная | Назначение |
|---|---|
| `PORT`, `APP_ENV` | порт и окружение |
| `POSTGRES_URL` | база данных |
| `INFLUX_URL`, `INFLUX_TOKEN`, `INFLUX_ORG`, `INFLUX_BUCKET` | метрики |
| `RABBITMQ_URL` | общая шина с агентами |
| `JWT_SECRET`, `JWT_ACCESS_TTL`, `JWT_REFRESH_TTL`, `BCRYPT_COST` | токены и пароли |
| `CORS_ALLOWED_ORIGINS` | должен содержать адрес панели |
| `AGENT_TOKEN` | общий секрет воркеров для heartbeat |
| `RATE_LIMIT_RPS`, `RATE_LIMIT_BURST` | лимит запросов |

## Роуты (`/api/v1`)

- `POST /auth/register`, `POST /auth/login` → `{data: {user, access_token, refresh_token, expires_at}}`
- `POST /auth/refresh` (`{refresh_token}`) → новая пара токенов, старый инвалидируется
- `GET /auth/me`, `POST /auth/logout` — по `Authorization: Bearer`
- `GET/POST/DELETE /servers`, `GET /users...` — воркеры и пользователи (по Bearer, часть — только `admin`)
- `POST /agents/heartbeat` — heartbeat воркеров (по общему `AGENT_TOKEN`), `GET /agents` — список серверов для карты (по Bearer)
- Админка (всё — Bearer + роль `admin`):
  `GET /users`, `POST /users` (`{login, password, role, plan_id}`),
  `PATCH /users/:id/role|block|plan` (`{blocked}`, `{plan_id, expires}`),
  `GET /admin/plans`, `GET /admin/containers`,
  `POST /admin/containers/:id/{stop|restart|remove}`, `POST /admin/servers/:id/reboot` (запуск чужих контейнеров админу недоступен — только администрирование)
  (действия уходят задачей в очередь `agent.<id>`, статусы возвращаются событиями в `events`)
- Контейнеры в стиле docker run: `POST /containers/check` (`{app_name, repo_url, node_id}` → 202, воркер клонирует и проверяет), `GET /containers/check/:id` (статус проверки для опроса), `POST /containers/run` (`{app_name, repo_url?, image?, container_name?, node_id, ports[], volumes[], env{}}` → сборка и запуск), `POST /containers/refresh` (`{app_name}` → стоп, свежий pull, пересборка, старт по сохранённым параметрам), `DELETE /containers/:id` (удалить свой контейнер → 202, запись исчезает по `container.removed`), `POST /containers/:id/start` (docker restart поднимает и остановленные), `POST /containers/:id/stop`, `GET /containers/mine` (свои контейнеры), `GET /containers/:id/logs` (прокси хвоста логов через воркер), `GET /containers/:id/spec` (полные параметры с воркера), `GET /containers/:id/build-log` (лог сборки с воркера, пока статус `building`), `GET /containers/:id/build-log/stream` (то же по WebSocket: токен в `?token=`, query в логах затирается)
- Тома: `GET /volumes`, `POST /volumes` (`{name, node_id, size_gb}`), `DELETE /volumes/:name?node=` (удаляет и на воркере); именованные тома из запусков подхватываются сами
- Env-конфиги: `GET /env-configs`, `POST`, `PATCH /:id`, `DELETE /:id` (до 10 на пользователя, лимит проверяет бэкенд)
- Квоты пользователя: `GET /quotas` → использовано/лимиты тарифа (приложения, порты, env, место); порты контейнеров хранятся в таблице и суммируются
- Файлы пользователя на воркерах (проксируется через очередь, владелец из токена): `GET /files?node=&path=`, `POST /files/mkdir`, `POST /files/upload` (multipart, до 20 МБ, заменяет), `GET /files/download`, `DELETE /files?node=&path=` (файл или папка целиком)
- Место пользователя: `GET /storage/usage` → `{containers_gb, files_gb, total_gb}` (сумма по всем воркерам из heartbeat)

## Первый вход

При старте с пустой таблицей `users` создаётся админ из `.env`:

- Логин: значение `ADMIN_LOGIN` (`admin`)
- Пароль: значение `ADMIN_PASSWORD` (`admin123`)

Поменяйте пароль после первого входа. Заблокированные пользователи получают 403 при логине.

## Безопасность

- Пароли — только bcrypt-хэш (`pkg/security`), открытых паролей в БД нет.
- Access-токен живёт 15 минут; refresh одноразовый: хранится в памяти процесса, повторное использование старого отклоняется и гасит цепочку.
- Logout удаляет refresh из памяти. Заголовки безопасности включены всегда, HSTS — в проде.
- `JWT_SECRET` в проде — длинная случайная строка, не значение из примера.

## RabbitMQ (топология `dockrunner`, direct)

| Очередь | Кто пишет | Кто читает | Содержимое |
|---|---|---|---|
| `agent.<id>` | централь | агент воркера | задачи `deploy`/`stop`/`restart`/`remove` |
| `events` | агенты | централь | `deploy.completed`, `deploy.failed`, `container.stopped`… |
| `notifications` | зарезервирована | — | — |

Задача деплоя: `{type, deployment_id, app_id, app_name, image, ports[], volumes[], env{}, restart_policy}`.

## Пользователи RabbitMQ

Да, отдельный пользователь под приложение — обязательно:
- Не `guest` (он только с localhost) и не один пароль на все сервисы — у Postgres, RabbitMQ и остальных свои учётки.
- Для MVP достаточно одного app-пользователя (например, `dockrunner`) и на централь, и на агенты — обе стороны уже ходят под одним `RABBITMQ_URL`.
- Виртуалхост лучше отдельный, не дефолтный `/`: `amqp://dockrunner:pass@host:5672/dockrunner`.
- Права минимум: централи — чтение/запись очередей `agent.*` и `events`, агентам — своя `agent.<id>` + запись в `events`:
  `rabbitmqctl set_permissions -p dockrunner dockrunner "^(agent\.|events)" "^(agent\.|events)" "^(agent\.|amq\.default|events)"`
- Админка веб-морды (`15672`) — отдельная учётка с тегом `administrator`, не app-пользователь.

## Диагностика: проверка висит, воркер отработал

1. Агент должен залогировать `event published type=prepare.completed`.
2. Централь должна залогировать `prepare resolved deployment=<id>` — если его нет: в логе старта ищите `events consumer skipped` (нет Postgres — consumer не работает) или `events consumer failed`.
3. Централь и агент обязаны смотреть в **один и тот же** брокер (`RABBITMQ_URL` с обеих сторон); иначе задачи уходят в одну очередь, а события ждут в другой.
4. Рестарт централи стирает in-memory проверки — фронт получит 404 и покажет «создайте проверку заново».

## Блокировка операций (`internal/oplock`)

Одна операция на приложение (`владелец/приложение`): `check`, `deploy`, `refresh`, `stop/restart/remove`. Повторная попытка, пока идёт первая, — HTTP 409 с текстом «сейчас выполняется операция …». Замок снимается терминальным событием (`deploy.completed/failed`, `op.failed`, `container.*`) или протуханием проверки. Событие `op.failed` от воркера (операция по несуществующему деплою) переводит запись в `error`.
Запись приложения появляется в `/mine` сразу при `POST /run` со статусом `building` — список не пуст, пока идёт сборка; `deploy.completed/failed` переводят её в `running`/`error`.

## Порты: диапазон и занятость

- Диапазон хост-портов: `PORTS_MIN`/`PORTS_MAX` (по умолчанию 3000–4000, отдаются фронту в `GET /quotas` как `ports_min/ports_max`).
- `POST /run` проверяет каждый хост-порт: вне диапазона — 400; порт уже занят (по записям узла в БД, кроме `stopped`/`error`) — 400 с именем приложения-держателя.
- Проверка и публикация задачи идут под мьютексом узла — два параллельных запуска не заберут один порт (в пределах одного процесса централи).
- Порты чужих процессов на воркере централь не видит — такой конфликт вернётся ошибкой Docker в `deploy.failed`.
Потребитель очереди не крутит poison-сообщения вечно: повторная доставка после ошибки дропается с логом.

## RabbitMQ с TLS (порт 5671)

Обычный образ из коробки шифрование не включает — его надо настроить:

1. Сертификаты (CN + SAN на localhost):
```bash
openssl genrsa -out ca.key 4096
openssl req -x509 -new -nodes -key ca.key -sha256 -days 825 -out ca.crt -subj "/CN=Dockrunner CA"
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr -subj "/CN=localhost"
printf "subjectAltName = DNS:localhost, IP:127.0.0.1" > server.ext
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 825 -sha256 -extfile server.ext
```
2. `rabbitmq.conf`:
```
listeners.tcp = none
listeners.ssl.1 = 5671
ssl_options.cacertfile = /etc/rabbitmq/certs/ca.crt
ssl_options.certfile = /etc/rabbitmq/certs/server.crt
ssl_options.keyfile = /etc/rabbitmq/certs/server.key
ssl_options.verify = verify_none
```
3. Запуск с монтированием сертов и конфига, проброс `-p 5668:5671`.
4. Проверка: `openssl s_client -connect 127.0.0.1:5668 -brief` должен показать цепочку.
5. В `.env` бэкенда и агента: `RABBITMQ_URL=amqps://...@127.0.0.1:5668/` + `RABBITMQ_TLS_SKIP_VERIFY=true` (только для самоподписанного серта).
