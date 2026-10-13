# Разворот агента на сервере

Агент ставится **на каждый сервер**, где будут крутиться контейнеры пользователей.
В общий `docker-compose.yml` он не входит: агенту нужен docker-сокет хоста
(`/var/run/docker.sock`), поэтому проще ставить бинарником или отдельным
контейнером с проброшенным сокетом.

## 1. Требования

- Linux x86_64, установленный Docker (контейнеры запускаются через него)
- Исходящие соединения до централи: `CENTRAL_URL` (443/80) и RabbitMQ `5672`
  (или `5671` для `amqps://`)
- Входящий порт `8090` (или свой `PORT`) — панель ходит туда за логами
  (`AGENT_PUBLIC_URL`, иначе логи/файлы недоступны)

## 2. Сборка из исходников

```bash
cd dockrunner_agent
go build -o agent ./cmd
```

Кросс-сборка с другой машины:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o agent ./cmd
```

## 3. Настройка (.env рядом с бинарником)

```bash
cp .env.example .env
```

Обязательное:

```env
AGENT_ID=worker-01            # уникальный id сервера
AGENT_NAME=fobos
AGENT_CITY=Frankfurt
CENTRAL_URL=https://panel.example.com
AGENT_TOKEN=тот-же-что-у-бэкенда   # 1 в 1 как AGENT_TOKEN централи
RABBITMQ_URL=amqp://dockrunner:dockrunner@central:5672/
AGENT_PUBLIC_URL=http://<ip-сервера>:8090
```

Необязательное: `AGENT_DESCRIPTION`, `AGENT_LAT/AGENT_LNG` (точка на карте),
`DOCKER_HOST` (пусто = `/var/run/docker.sock`), `HEARTBEAT_INTERVAL=15s`,
`SQLITE_PATH=./data/agent.db`, `WORKSPACE_DIR=./workspace`.

## 4. Запуск через systemd

```ini
# /etc/systemd/system/dockrunner-agent.service
[Unit]
Description=Dockrunner agent
After=docker.service network-online.target
Requires=docker.service

[Service]
User=root
WorkingDirectory=/opt/dockrunner-agent
ExecStart=/opt/dockrunner-agent/agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now dockrunner-agent
journalctl -u dockrunner-agent -f
```

Агент пишет heartbeat каждые `HEARTBEAT_INTERVAL`. Если в панели сервер
«офлайн» — смотри лог: обычно неверный `AGENT_TOKEN` или закрытый `5672`.

> Агент должен запускаться от пользователя с доступом к docker-сокету
> (обычно `root` или группа `docker`).

## 5. Вариант: агент в docker

```bash
docker build -t dockrunner-agent .
docker run -d --name agent --restart unless-stopped \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v agent-data:/data \
  -p 8090:8090 \
  --env-file .env \
  dockrunner-agent
```

## 6. Обновление

```bash
cd dockrunner_agent
git pull
go build -o agent ./cmd
sudo systemctl restart dockrunner-agent
```

При рестарте задачи не теряются: очередь RabbitMQ держит их до `ack`,
а локальные деплои лежат в `data/agent.db`. Выполняющаяся в момент рестарта
сборка может оборваться — её можно повторить кнопкой обновления.
