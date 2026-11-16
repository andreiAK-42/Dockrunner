# Dockrunner — инструкция по использованию

Dockrunner разворачивает студенческие ИТ-проекты в Docker-контейнерах на отдельных серверах-воркерах.

## Части системы

| Часть | Папка | Что делает |
|---|---|---|
| Веб-панель | `dockrunner` | React-интерфейс: приложения, домены, базы данных, уведомления, профиль |
| Центральный сервер | `dockrunner_backend` | Go API: пользователи, JWT-токены, Postgres/VictoriaMetrics/VictoriaLogs, RabbitMQ |
| Агент воркера | `dockrunner_agent` | Go-сервис на каждом сервере: запускает контейнеры, отдаёт логи, шлёт heartbeat |

Схема связей:

```
Панель ──HTTP──▶ Централь ──RabbitMQ──▶ Агент ──Docker──▶ контейнеры приложений
Панель ──HTTP/WS──▶ Агент (логи)        Агент ──HTTP──▶ Централь (heartbeat)
```

## Быстрый старт (демо без серверов)

1. Инфраструктура (postgres, rabbitmq, victoria-metrics, victoria-logs):
   `cp .env.example .env` → заполнить секреты → `docker compose -f docker-compose-services.yml up -d`.
2. Бэкенд + панель (читают тот же `.env`):
   `docker compose up -d --build` → `http://localhost:8080/health`.
   Отдельной разработкой: `cd dockrunner_backend && cp .env.example .env && go run ./cmd/main`
   (`.env` для `go run` — локальный, с `localhost`-адресами, см. `backend.md`),
   панель: `cd dockrunner && npm install && npm run dev` (без бэкенда работает на моках).
3. Агент на каждом сервере-воркере:
   `cp env.agent.example env.agent` → заполнить → `docker compose -f docker-compose-services-agent.yml up -d --build`.
   Детали — `agent.md` и `dockrunner_agent/DEPLOY.md`.

Порядок запуска: сначала инфраструктура, потом бэкенд/панель, агенты — в любой момент.

## Документация

- `backend.md` — центральный сервер: запуск, окружение, роуты, очереди.
- `frontend.md` — панель: запуск, страницы, API-слой, тарифы и лимиты.
- `agent.md` — воркер: установка на Ubuntu 22, Docker, задачи, логи, heartbeat.
