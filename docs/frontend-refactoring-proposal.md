# Предложение по рефакторингу фронтенда: разделение больших файлов

## Текущие большие файлы (>500 строк)

| Файл | Строк | Статус |
|------|-------|--------|
| `src/pages/CreateApplication/index.tsx` | **1239** | 🔴 Критично |
| `src/pages/ApplicationDetails/index.tsx` | 702 | 🟠 Нужно делить |
| `src/pages/ContainerDetails/index.tsx` | 695 | 🟠 Нужно делить |
| `src/components/ServerMap.tsx` | 428 | 🟡 На границе |

---

## Архитектурный анализ CreateApplication (1239 строк)

### Что внутри (грубый подсчёт):
- **~80 строк** — импорты, типы, константы, утилиты (mulberry32, randomFreePorts, portSourceLabel, nextFreeHostPort, waitForCheck, getNodeLoad и т.д.)
- **~200 строк** — состояние (20+ `useState`, `useMemo`, `useEffect`)
- **~400 строк** — хендлеры (handleCheck, handleAutoFill, handleDeploy, handleAutoFillApply, fillFromConfig, updatePort/Volume/Env и т.д.)
- **~550 строк** — JSX (6 секций формы + 2 диалога + футер + Snackbar)

### Проблемы:
1. **God Component** — всё в одном файле: состояние, логика, UI, диалоги
2. **Смешанные ответственности** — порты, тома, env vars, проверка репо, деплой, автозаполнение
3. **Сложно тестировать** — нельзя протестировать логику портов отдельно от UI
4. **Дублирование** — похожие паттерны для ports/volumes/env (add/remove/update)

---

## Предлагаемая структура

### 1. Вынести утилиты в отдельные файлы

```
src/utils/
├── ports.ts              # randomFreePorts, nextFreeHostPort, parsePort, buildMappedPorts
├── check.ts              # waitForCheck, isCheckFinished, CHECK_POLL_INTERVAL_MS
├── nodes.ts              # getWorkerName, getNodeLoad, findLeastLoadedNode
└── docker.ts             # portSourceLabel, buildMappedVolumes, buildEnvMap
```

### 2. Вынести кастомные хуки

```
src/hooks/
├── usePortManager.ts        # состояние ports, suggestions, add/remove/update, автоподбор
├── useVolumeManager.ts      # состояние volumes, add/remove/update
├── useEnvManager.ts         # состояние envRows, appliedKeys, fillFromConfig, auto-fill
├── useRepoCheck.ts          # handleCheck, handleCancelCheck, checking, elapsed, check status
├── useAutoFill.ts           # handleAutoFill, handleAutoFillApply, handleAutoFillCancel, dialog state
├── useDeployment.ts         # handleDeploy, deploying, requestSubmitted, validation
└── usePricing.ts            # priceEstimate, rates, cpu/memory/disk/duration
```

### 3. Вынести UI-секции в компоненты

```
src/pages/CreateApplication/
├── index.tsx                    # ~150 строк — только композиция
├── sections/
│   ├── BasicInfoSection.tsx     # appName, repoUrl, branch, AutoFillButton
│   ├── DeploymentTargetSection.tsx  # radio automatic/manual, ServerMap, node selection
│   ├── PortsSection.tsx         # PortsSection + usePortManager
│   ├── VolumesSection.tsx       # VolumesSection + useVolumeManager
│   ├── EnvVarsSection.tsx       # EnvVarsSection + useEnvManager
│   └── ResourcesSection.tsx     # cpu, memory, disk, duration, pricing
├── dialogs/
│   ├── BuildModeDialog.tsx      #.build mode + services selection
│   └── AutoFillDialog.tsx       # branch + build mode + services + env + ports
├── FooterActions.tsx            # Cancel / Check / Deploy кнопки
└── types.ts                     # локальные типы PortRow, VolumeRow, EnvRow, MappedPort...
```

### 4. Принципы разделения

| Принцип | Применение |
|---------|------------|
| **Single Responsibility** | Каждый хук/компонент — одна задача |
| **Colocation** | Хуки рядом с компонентами, которые их используют |
| **Extract early** | Утилиты — в `src/utils`, переиспользуемые хуки — в `src/hooks` |
| **Types together** | Типы рядом с компонентами/хуками, которые их используют |

---

## Пример: CreateApplication после рефакторинга (~150 строк)

```tsx
// src/pages/CreateApplication/index.tsx
import { BasicInfoSection } from './sections/BasicInfoSection'
import { DeploymentTargetSection } from './sections/DeploymentTargetSection'
import { PortsSection } from './sections/PortsSection'
import { VolumesSection } from './sections/VolumesSection'
import { EnvVarsSection } from './sections/EnvVarsSection'
import { ResourcesSection } from './sections/ResourcesSection'
import { FooterActions } from './FooterActions'
import { BuildModeDialog } from './dialogs/BuildModeDialog'
import { AutoFillDialog } from './dialogs/AutoFillDialog'
import { useRepoCheck } from './hooks/useRepoCheck'
import { useAutoFill } from './hooks/useAutoFill'
import { useDeployment } from './hooks/useDeployment'
import { usePricing } from './hooks/usePricing'

export default function CreateApplication() {
  const { check, isReady, checking, elapsed, handleCheck, handleCancelCheck } = useRepoCheck()
  const { showAutoFillDialog, handleAutoFill, handleAutoFillApply, handleAutoFillCancel, ... } = useAutoFill()
  const { deploying, requestSubmitted, requestId, handleDeploy } = useDeployment()
  const { priceEstimate } = usePricing()

  return (
    <PageLayout>
      <BasicInfoSection onAutoFill={handleAutoFill} />
      <DeploymentTargetSection />
      <PortsSection />
      <VolumesSection />
      <EnvVarsSection />
      <ResourcesSection priceEstimate={priceEstimate} />
      <FooterActions
        isReady={isReady}
        checking={checking}
        elapsed={elapsed}
        onCheck={handleCheck}
        onCancelCheck={handleCancelCheck}
        onDeploy={handleDeploy}
        deploying={deploying}
        requestSubmitted={requestSubmitted}
        requestId={requestId}
      />
      <BuildModeDialog ... />
      <AutoFillDialog ... />
    </PageLayout>
  )
}
```

---

## ApplicationDetails (702) и ContainerDetails (695)

### ApplicationDetails — предложение:
```
src/pages/ApplicationDetails/
├── index.tsx
├── sections/
│   ├── AppHeader.tsx           # название, статус, действия
│   ├── InfoSection.tsx         # основная инфа, репо, ветка
│   ├── ContainersList.tsx      # таблица контейнеров
│   ├── PortsSection.tsx        # порты
│   ├── VolumesSection.tsx      # тома
│   ├── EnvVarsSection.tsx      # env vars
│   └── ResourcesSection.tsx    # ресурсы, тарифы
├── dialogs/
│   ├── EditContainerDialog.tsx
│   ├── RecreateDialog.tsx
│   └── RequestUpdateDialog.tsx
└── hooks/
    ├── useApplication.ts
    └── useContainerActions.ts
```

### ContainerDetails — предложение:
```
src/pages/ContainerDetails/
├── index.tsx
├── sections/
│   ├── ContainerHeader.tsx
│   ├── SpecSection.tsx
│   ├── LogsSection.tsx
│   ├── StatsSection.tsx
│   └── ActionsSection.tsx
├── dialogs/
│   ├── RecreateDialog.tsx
│   └── RequestUpdateDialog.tsx
└── hooks/
    ├── useContainer.ts
    └── useContainerActions.ts
```

---

## План миграции (поэтапно, без breaking changes)

### Этап 1: Утилиты (низкий риск)
1. Создать `src/utils/ports.ts`, `check.ts`, `nodes.ts`, `docker.ts`
2. Перенести туда чистые функции
3. Обновить импорты в CreateApplication

### Этап 2: Хуки для менеджеров состояния (средний риск)
1. `usePortManager`, `useVolumeManager`, `useEnvManager`
2. По одному, проверяя работоспособность после каждого

### Этап 3: UI-секции (средний риск)
1. Вынести секции в компоненты, передавать состояние через пропсы
2. Использовать хуки из этапа 2 внутри секций

### Этап 4: Диалоги (низкий риск)
1. `BuildModeDialog`, `AutoFillDialog` — изолированные компоненты

### Этап 5: ApplicationDetails / ContainerDetails
Аналогично этапам 1-4.

---

## Оценка усилий

| Этап | Файлов | Время | Риск |
|------|--------|-------|------|
| 1. Утилиты | 4 | ~30 мин | 🟢 Низкий |
| 2. Хуки менеджеров | 3 | ~1-2ч | 🟡 Средний |
| 3. UI-секции | 6 | ~2-3ч | 🟡 Средний |
| 4. Диалоги | 2 | ~1ч | 🟢 Низкий |
| 5. Details страницы | ~10 | ~4-6ч | 🟡 Средний |
| **Итого** | ~25 | ~8-12ч | |

---

## Правила на будущее (ESLint / конвенции)

1. **max-lines-per-file: 500** — линтер будет ругаться
2. **max-lines-per-function: 100** — для хендлеров
3. **Компонент > 200 строк** → выносить подкомпоненты
4. **Хук > 150 строк** → разделять на мелкие хуки
5. **Колокация** — связанные файлы в одной папке (`CreateApplication/sections/`, `CreateApplication/dialogs/`)