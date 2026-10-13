# Быстрый старт

## 1. Создайте приложение

Перейдите в **Applications → Create Application**, укажите:

- Название: `my-awesome-app`
- Источник: `GitHub Repository` или `Dockerfile`
- Порты: выберите свободный из пула `3000-3004`
- Volumes: добавьте маунт `/data` если нужно
- Env: выберите сохранённый конфиг из **Env Configs**

Нажмите **Deploy Application**.

## 2. Проверьте деплой

Откройте **Deployments** — статус `Running` означает успех.

## 3. Откройте контейнер

В **Applications → ваш проект → Containers** кликните по строке контейнера.

Там доступны:

- **Логи** (`docker logs`) с поиском и Follow
- **Параметры запуска** — полная команда `docker run -d --env ... -p ... -v ...`

```bash
docker run -d --name my-app -p 3000:80 --env NODE_ENV=production nginx:1.27
```

![Скриншот контейнера](../docs/screenshots/container.png)

## 4. Домены

В **Domains** добавьте домен, Caddy выпустит сертификат автоматически.
