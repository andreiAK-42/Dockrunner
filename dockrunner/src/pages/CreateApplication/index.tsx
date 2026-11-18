import { useCreateApplication } from '../../hooks/useCreateApplication'
import { useNavigate } from 'react-router-dom'
import { Box, Typography, Card, CardContent, Grid, Alert, Button, Stack, Divider, Snackbar, FormControl, InputLabel, Select, MenuItem, RadioGroup, FormControlLabel, Radio, FormHelperText, TextField, CircularProgress } from '@mui/material'
import { ArrowLeft, Save, GitBranch, RefreshCw } from 'lucide-react'
import ServerMap from '../../components/ServerMap'
import { getWorkerName } from '../../utils/nodes'
import EditableList from '../../components/forms/EditableList'
import PortRow from '../../components/forms/PortRow'
import VolumeRow from '../../components/forms/VolumeRow'
import EnvRow from '../../components/forms/EnvRow'
import { parsePort, randomFreePorts } from '../../utils/ports'
import { BuildModeDialog } from './dialogs/BuildModeDialog'
import { AutoFillDialog } from './dialogs/AutoFillDialog'
import styles from './CreateApplication.module.scss'

export default function CreateApplication() {
  const navigate = useNavigate()

  const {
    // State
    appName,
    setAppName,
    repoUrl,
    setRepoUrl,
    branch,
    setBranch,
    deploymentMode,
    setDeploymentMode,
    selectedNodeId,
    setSelectedNodeId,

    // Form arrays
    ports,
    volumes,
    envRows,

    // Env config
    envConfigId,
    configs,
    fillFromConfig,

    // Check state
    check,
    autoFillCheck,
    showAutoFillDialog,
    autoFillBranch,
    setAutoFillBranch,
    autoFillEnvKeys,
    setAutoFillEnvKeys,
    autoFillPortIndices,
    setAutoFillPortIndices,

    // Build mode
    buildMode,
    selectedServices,
    showBuildModeDialog,
    pendingBuildMode,
    setPendingBuildMode,
    pendingServices,
    setPendingServices,

    // Loading states
    checking,
    elapsed,
    deploying,
    formError,
    toast,
    setToast,
    requestSubmitted,
    requestId,

    // Resource requests
    cpuCores,
    setCpuCores,
    memoryGB,
    setMemoryGB,
    diskGB,
    setDiskGB,
    durationHours,
    setDurationHours,
    priceEstimate,

    // Nodes and ports
    onlineNodes,
    recommendedNode,
    busyPorts,
    freeCount,
    portsRange,
    suggestions,
    regenerateSuggestions,

    // Actions
    handleCheck,
    handleCancelCheck,
    handleAutoFill,
    handleAutoFillApply,
    handleAutoFillCancel,
    handleBuildModeConfirm,
    handleBuildModeCancel,
    handleDeploy,
  } = useCreateApplication()

  const effectiveNodeId = deploymentMode === 'automatic' ? recommendedNode?.id || '' : selectedNodeId
  const isReady = check?.status === 'ready'

  const activeServices = buildMode === 'compose' ? selectedServices : []
  const serviceCount = buildMode === 'compose' ? activeServices.length : 1

  // Add port handler
  const handleAddPort = () => {
    const usedHosts = new Set([
      ...busyPorts.filter((busy) => busy.nodeId === effectiveNodeId).map((busy) => busy.port),
      ...ports.items.map((row) => parsePort(row.host)).filter((port): port is number => port !== null),
      ...suggestions,
    ])
    const [next] = randomFreePorts(1, portsRange.min, portsRange.max, usedHosts, Date.now())
    ports.addItem({ host: next !== undefined ? String(next) : '', container: '' })
  }

  return (
    <Box className={styles.page}>
      <Box className={styles.header}>
        <Button startIcon={<ArrowLeft size={16} />} onClick={() => navigate('/applications')} className={styles.header__back}>
          Назад
        </Button>
        <Typography variant="h5" className={styles.header__title}>Создание приложения</Typography>
      </Box>

      <Card>
        <CardContent className={styles.cardInner}>
          <Grid container spacing={4}>
            {/* Section 1: Basic Info */}
            <Grid size={{ xs: 12 }}>
              <Typography variant="h6" className={styles.sectionTitle}>1. Основная информация</Typography>
              <Grid container spacing={3}>
                <Grid size={{ xs: 12 }}>
                  <TextField fullWidth label="Название приложения" placeholder="например, my-awesome-app" value={appName} onChange={(e) => setAppName(e.target.value)} helperText="Название станет именем контейнера" />
                </Grid>
                <Grid size={{ xs: 12 }}>
                  <Box className={styles.repoRow}>
                    <TextField
                      className={styles.repoField}
                      fullWidth
                      label="Ссылка на Git-репозиторий"
                      placeholder="https://github.com/org/repo"
                      value={repoUrl}
                      onChange={(e) => setRepoUrl(e.target.value)}
                      size="small"
                    />
                    <Button
                      className={styles.autoFillBtn}
                      variant="outlined"
                      startIcon={<RefreshCw size={16} />}
                      onClick={handleAutoFill}
                      disabled={checking || !repoUrl.trim()}
                      size="small"
                    >
                      Автозаполнение
                    </Button>
                  </Box>
                  <Typography className={styles.repoHint}>
                    Сервер склонирует проект в папку пользователя и проверит Dockerfile или docker-compose в корне
                  </Typography>
                </Grid>
                <Grid size={{ xs: 12 }}>
                  {check?.branches && check.branches.length > 0 ? (
                    <FormControl fullWidth size="small">
                      <InputLabel id="branch-label">Ветка Git</InputLabel>
                      <Select
                        labelId="branch-label"
                        value={branch}
                        label="Ветка Git"
                        onChange={(e) => setBranch(e.target.value)}
                        displayEmpty
                      >
                        {check.branches.map((b) => (
                          <MenuItem key={b} value={b}>{b}</MenuItem>
                        ))}
                      </Select>
                      <FormHelperText>Выберите ветку из репозитория</FormHelperText>
                    </FormControl>
                  ) : (
                    <TextField fullWidth label="Ветка Git" placeholder="main" value={branch} onChange={(e) => setBranch(e.target.value)} helperText="Обязательное поле — укажите ветку для деплоя (например, main, master, develop)" />
                  )}
                </Grid>
              </Grid>
            </Grid>
            <Grid size={{ xs: 12 }}><Divider /></Grid>

            {/* Section 2: Deployment Target */}
            <Grid size={{ xs: 12 }}>
              <Typography variant="h6" className={styles.sectionTitle}>2. Цель деплоя и проверка</Typography>
              <FormControl>
                <RadioGroup row value={deploymentMode} onChange={(e) => setDeploymentMode(e.target.value as 'automatic' | 'manual')}>
                  <FormControlLabel value="automatic" control={<Radio />} label="Автоматически — планировщик выберет узел" />
                  <FormControlLabel value="manual" control={<Radio />} label="Выбрать узел вручную" />
                </RadioGroup>
              </FormControl>
              {onlineNodes.length === 0 && (
                <Alert severity="warning" sx={{ mb: 1.5 }}>Нет доступных серверов — проверьте соединение с backend и heartbeat воркеров.</Alert>
              )}
              {deploymentMode === 'automatic' && recommendedNode && (
                <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>Рекомендуемый узел: <b>{recommendedNode.name} ({recommendedNode.city})</b></Typography>
              )}
              {deploymentMode === 'automatic' && recommendedNode && (
                <Box className={styles.mapWrap}><ServerMap nodes={onlineNodes} selectedNodeId={recommendedNode.id} /></Box>
              )}
              {deploymentMode === 'manual' && (
                <Stack spacing={1.5} sx={{ mb: 1.5 }}>
                  {onlineNodes.map((node) => (
                    <Button
                      key={node.id}
                      variant={selectedNodeId === node.id ? 'contained' : 'outlined'}
                      onClick={() => setSelectedNodeId(node.id)}
                      sx={{ justifyContent: 'flex-start' }}
                    >
                      {node.name} ({node.city}) — свободно CPU {(node.cpu.total - node.cpu.used).toFixed(1)}, RAM {(node.memory.total - node.memory.used).toFixed(1)} ГБ
                    </Button>
                  ))}
                </Stack>
              )}
              {deploymentMode === 'manual' && (
                <Box className={styles.mapWrap}><ServerMap nodes={onlineNodes} selectedNodeId={selectedNodeId} onSelectNode={(node) => setSelectedNodeId(node.id)} /></Box>
              )}
              {deploymentMode === 'manual' && selectedNodeId && <Typography variant="body2" className={styles.alert} color="text.secondary">Выбран узел: <b>{getWorkerName(onlineNodes, selectedNodeId)}</b></Typography>}
              {formError && <Alert severity="error" sx={{ mt: 2 }}>{formError}</Alert>}
            </Grid>
            <Grid size={{ xs: 12 }}><Divider /></Grid>

            {/* Section 3: Ports */}
            <Grid size={{ xs: 12 }}>
              <Typography variant="h6" className={styles.sectionTitle}>3. Порты</Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
                Хост-порт выбирается из свободных на узле (диапазон {portsRange.min}–{portsRange.max}). Свободно по лимиту: {freeCount}.
              </Typography>
              <EditableList
                items={ports.items}
                renderItem={(item, index, onUpdate, onRemove) => (
                  <PortRow
                    row={item}
                    index={index}
                    suggestions={suggestions}
                    busyPorts={busyPorts}
                    portsRange={portsRange}
                    effectiveNodeId={effectiveNodeId}
                    onUpdate={onUpdate}
                    onRemove={onRemove}
                    isLast={index === ports.items.length - 1}
                  />
                )}
                onAdd={handleAddPort}
                onRemove={ports.removeItem}
                onUpdate={ports.updateItem}
                addButtonLabel="Добавить порт"
                emptyMessage="Нет портов"
              />
              <Stack direction="row" spacing={1.5}>
                <Button variant="outlined" size="small" startIcon={<RefreshCw size={14} />} onClick={regenerateSuggestions} sx={{ alignSelf: 'flex-start' }}>
                  Другие варианты
                </Button>
              </Stack>
            </Grid>
            <Grid size={{ xs: 12 }}><Divider /></Grid>

            {/* Section 4: Volumes */}
            <Grid size={{ xs: 12 }}>
              <Typography variant="h6" className={styles.sectionTitle}>4. Тома</Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>Локальный путь — только внутри вашей папки на сервере, наружу выйти нельзя. Просто имя — именованный том.</Typography>
              <EditableList
                items={volumes.items}
                renderItem={(item, index, onUpdate, onRemove) => (
                  <VolumeRow
                    row={item}
                    index={index}
                    onUpdate={onUpdate}
                    onRemove={onRemove}
                    isLast={index === volumes.items.length - 1}
                  />
                )}
                onAdd={() => volumes.addItem({ hostPath: '', containerPath: '' })}
                onRemove={volumes.removeItem}
                onUpdate={volumes.updateItem}
                addButtonLabel="Добавить том"
                emptyMessage="Нет томов"
              />
            </Grid>
            <Grid size={{ xs: 12 }}><Divider /></Grid>

            {/* Section 5: Resources */}
            <Grid size={{ xs: 12 }}>
              <Typography variant="h6" className={styles.sectionTitle}>6. Ресурсы и период</Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>Укажите необходимые ресурсы. Стоимость рассчитается автоматически по тарифам узла.</Typography>
              <Grid container spacing={3}>
                <Grid size={{ xs: 12, sm: 6 }}>
                  <TextField fullWidth type="number" label="CPU (ядер)" value={cpuCores} onChange={(e) => setCpuCores(Math.max(0.1, parseFloat(e.target.value) || 0.1))} slotProps={{ htmlInput: { step: 0.1, min: 0.1 } }} />
                </Grid>
                <Grid size={{ xs: 12, sm: 6 }}>
                  <TextField fullWidth type="number" label="Оперативная память (ГБ)" value={memoryGB} onChange={(e) => setMemoryGB(Math.max(0.1, parseFloat(e.target.value) || 0.1))} slotProps={{ htmlInput: { step: 0.1, min: 0.1 } }} />
                </Grid>
                <Grid size={{ xs: 12, sm: 6 }}>
                  <TextField fullWidth type="number" label="Диск (ГБ)" value={diskGB} onChange={(e) => setDiskGB(Math.max(0.1, parseFloat(e.target.value) || 0.1))} slotProps={{ htmlInput: { step: 0.1, min: 0.1 } }} />
                </Grid>
                <Grid size={{ xs: 12, sm: 6 }}>
                  <TextField fullWidth type="number" label="Период запуска (часов)" value={durationHours} onChange={(e) => setDurationHours(Math.max(1, parseInt(e.target.value) || 1))} slotProps={{ htmlInput: { step: 1, min: 1 } }} />
                </Grid>
              </Grid>
              <Alert severity="info" sx={{ mt: 2 }}>
                После проверки репозитория вы отправите заявку администратору. После одобрения приложение запустится на {durationHours} ч, затем автоматически остановится. Стоимость спишется с баланса.
                {priceEstimate !== null && <> Ориентировочно: {priceEstimate.toFixed(2)} руб за {serviceCount} конт.</>}
              </Alert>
            </Grid>
            <Grid size={{ xs: 12 }}><Divider /></Grid>

            {/* Section 6: Environment Variables */}
            <Grid size={{ xs: 12 }}>
              <Typography variant="h6" className={styles.sectionTitle}>5. Переменные окружения</Typography>
              <FormControl fullWidth size="small" sx={{ mb: 1.5 }}>
                <InputLabel>Заполнить из конфига</InputLabel>
                <Select value={envConfigId} label="Заполнить из конфига" onChange={(e) => fillFromConfig(e.target.value)}>
                  <MenuItem value=""><em>— Не использовать —</em></MenuItem>
                  {configs.map((c) => <MenuItem key={c.id} value={c.id}>{c.name} — {c.vars.length} перем.</MenuItem>)}
                </Select>
              </FormControl>
              <EditableList
                items={envRows.items}
                renderItem={(item, index, onUpdate, onRemove) => (
                  <EnvRow
                    row={item}
                    index={index}
                    onUpdate={onUpdate}
                    onRemove={onRemove}
                    isLast={index === envRows.items.length - 1}
                  />
                )}
                onAdd={() => envRows.addItem({ key: '', value: '' })}
                onRemove={envRows.removeItem}
                onUpdate={envRows.updateItem}
                addButtonLabel="Добавить переменную"
                emptyMessage="Нет переменных окружения"
              />
            </Grid>
          </Grid>
        </CardContent>
      </Card>

      <Box className={styles.footerActions}>
        <Button variant="outlined" onClick={() => navigate('/applications')}>Отмена</Button>
        {requestSubmitted ? (
          <Alert severity="success" sx={{ flex: 1, textAlign: 'center' }}>
            Заявка #{requestId?.slice(0, 8)} отправлена на одобрение. Администратор рассмотрит её в ближайшее время.
          </Alert>
        ) : isReady ? (
          <Button variant="contained" color="primary" startIcon={<Save size={18} />} disabled={deploying} onClick={handleDeploy}>
            {deploying ? 'Отправка заявки…' : 'Отправить на рассмотрение'}
          </Button>
        ) : checking ? (
          <>
            <Typography variant="body2" color="text.secondary" sx={{ alignSelf: 'center' }}>
              Ожидание ответа от {getWorkerName(onlineNodes, effectiveNodeId)}… {elapsed}с
            </Typography>
            <Button variant="contained" color="primary" disabled startIcon={<CircularProgress size={18} color="inherit" />}>Проверка…</Button>
            <Button variant="outlined" onClick={handleCancelCheck}>Отмена</Button>
          </>
        ) : (
          <Button
            variant="contained"
            color="primary"
            startIcon={<GitBranch size={18} />}
            onClick={handleCheck}
            disabled={!appName.trim() || !repoUrl.trim() || !branch.trim() || !effectiveNodeId}
          >
            Проверить репозиторий
          </Button>
        )}
      </Box>

      {/* Build Mode Dialog */}
      <BuildModeDialog
        open={showBuildModeDialog}
        onClose={handleBuildModeCancel}
        check={check}
        pendingBuildMode={pendingBuildMode}
        setPendingBuildMode={setPendingBuildMode}
        pendingServices={pendingServices}
        setPendingServices={setPendingServices}
        portsRange={portsRange}
        onConfirm={handleBuildModeConfirm}
      />

      {/* Auto Fill Dialog */}
      <AutoFillDialog
        open={showAutoFillDialog}
        onClose={handleAutoFillCancel}
        autoFillCheck={autoFillCheck}
        autoFillBranch={autoFillBranch}
        setAutoFillBranch={setAutoFillBranch}
        autoFillEnvKeys={autoFillEnvKeys}
        setAutoFillEnvKeys={setAutoFillEnvKeys}
        autoFillPortIndices={autoFillPortIndices}
        setAutoFillPortIndices={setAutoFillPortIndices}
        pendingBuildMode={pendingBuildMode}
        setPendingBuildMode={setPendingBuildMode}
        pendingServices={pendingServices}
        setPendingServices={setPendingServices}
        portsRange={portsRange}
        onApply={handleAutoFillApply}
      />

      <Snackbar
        open={toast !== null}
        autoHideDuration={6000}
        onClose={() => setToast(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      >
        <Alert severity={toast?.severity || 'info'} onClose={() => setToast(null)} sx={{ width: '100%' }}>
          {toast?.message}
        </Alert>
      </Snackbar>
    </Box>
  )
}