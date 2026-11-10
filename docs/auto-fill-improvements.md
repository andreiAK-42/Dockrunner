# Предложения по улучшению автозаполнения (Future Improvements)

## Текущее состояние
- Автозаполнение работает: проверяет репозиторий, определяет ветки, env vars, порты
- В диалоге есть выбор способа сборки: **Dockerfile** или **Docker Compose**
- Порты фильтруются по выбранному режиму (`source === 'dockerfile'` или `'compose'`)
- При Compose показываются чекбоксы сервисов
- **Env vars показываются все сразу** (из .env/.env.example), без разделения по источнику

---

## Проблема с env vars
Сейчас бэкенд (`dockrunner_agent/internal/worker/worker.go` → `DetectEnvFiles`) сканирует `.env` и `.env.example` и возвращает **объединённый** мап `detected_env_vars: map[string]string`.

Нет информации, из какого именно файла пришла переменная, и к какому способу сборки она относится.

---

## Предлагаемые изменения в бэкенде

### 1. Разделить env vars по файлам-источникам
В структуре `Event` (worker.go) и `AgentEvent` (admin/service.go) заменить:
```go
DetectedEnvVars map[string]string
```
на:
```go
DetectedEnvVars struct {
    FromDotEnv       map[string]string `json:"from_dot_env"`        // .env
    FromDotEnvExample map[string]string `json:"from_dot_env_example"` // .env.example
}
```

### 2. В API (containers/service.go) пробросить раздельно
```go
type CheckStatus struct {
    ...
    DetectedEnvVarsFromDotEnv       map[string]string `json:"detected_env_vars_from_dot_env"`
    DetectedEnvVarsFromDotEnvExample map[string]string `json:"detected_env_vars_from_dot_env_example"`
}
```

### 3. На фронтенде (CreateApplication)
В диалоге автозаполнения показывать две секции:
- **Переменные из `.env`** (обычно продакшн/секреты — пользователь решает, брать или нет)
- **Переменные из `.env.example`** (шаблон — можно безопасно подставить)

Или: одна секция с колонкой "Источник" (badge `.env` / `.env.example`).

---

## Дополнительные идеи

### А. Порты — привязка к сервису в Compose
Сейчас у порта есть `service` (имя сервиса в compose). Можно в UI группировать порты по сервисам:
```
web:
  - 80/tcp → авто-порт
  - 443/tcp → авто-порт
db:
  - 5432/tcp → авто-порт
```

### Б. Автозаполнение ресурсов (CPU/RAM/диск)
Если в compose есть `deploy.resources.limits` — подставлять в форму.

### В. Healthcheck / Restart policy
Из Dockerfile/Compose вытаскивать `HEALTHCHECK`, `restart:` и предлагать в форме.

### Г. Volumes
Из `VOLUME` в Dockerfile и `volumes:` в compose — предлагать тома.

---

## Приоритеты
| # | Задача | Сложность | Польза |
|---|--------|-----------|--------|
| 1 | Разделить env vars по `.env` / `.env.example` в бэкенде | Средне | Высокая |
| 2 | Группировка портов по сервисам в UI | Низкая | Средняя |
| 3 | Автозаполнение ресурсов из compose | Средне | Средняя |
| 4 | Healthcheck / restart policy | Низкая | Низкая |
| 5 | Volumes из Dockerfile/Compose | Средне | Средняя |

---

## Заметки по реализации
- Изменения затрагивают: `agent/worker`, `admin/service`, `containers/service`, `consumer/events`, `api/containers.ts`, `CreateApplication.tsx`
- Миграция БД не нужна (данные только в runtime events)
- Совместимость: старые события без новых полей — обрабатывать как пустые мапы