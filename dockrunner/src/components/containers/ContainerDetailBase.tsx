import { Box, Card, CardContent, Alert, Snackbar, Stack, Typography, Grid, Tooltip, Chip, Button } from '@mui/material'
import { Terminal, Box as BoxIcon, Server, Activity, Copy } from 'lucide-react'
import ContainerTabs from './ContainerTabs'
import ContainerHeader from './ContainerHeader'
import ContainerActions from './ContainerActions'
import DockerCommandEditor from './DockerCommandEditor'
import PortsList from './PortsList'
import VolumesList from './VolumesList'
import EnvConfigSelector from './EnvConfigSelector'
import PortQuota from '../../components/PortQuota'
import type { MineContainer, ContainerSpec } from '../../api/containers'
import type { EnvConfig } from '../../types/envConfig'
import { timeAgo } from '../../utils/time'
import { statusLabel } from '../../utils/status'
import { copyText } from '../../utils/clipboard'

interface ContainerDetailBaseProps {
  // Data
  record: MineContainer | null
  spec: ContainerSpec | null
  selectedEnvConfig: EnvConfig | undefined
  envConfigs: EnvConfig[]
  envConfigId: string
  setEnvConfigId: (id: string) => void
  displayedDockerCmd: string
  editedDockerCmd: string | null
  setEditedDockerCmd: (cmd: string | null) => void
  applyingCommand: boolean
  isBuilding: boolean
  followBuildLog: boolean
  setFollowBuildLog: (v: boolean) => void
  buildLog: string | null
  buildLogError: string
  buildLogBoxRef: React.RefObject<HTMLDivElement | null>
  logs: string[] | null
  logsError: string
  logsLive: boolean
  logsLoading: boolean

  // Status
  updating: boolean
  deleting: boolean
  confirmDelete: boolean
  acting: '' | 'start' | 'stop' | 'restart'
  busy: boolean
  syncing: boolean
  syncError: string | null
  syncSuccess: string | null
  updateMsg: { ok: boolean; text: string } | null

  // Actions
  verifyContainerStatus: () => Promise<void>
  handleDelete: () => Promise<void>
  handleRefresh: () => Promise<void>
  applyEditedCommand: () => Promise<void>
  handleOp: (action: 'start' | 'stop' | 'restart') => Promise<void>
  handleCopyLogs: () => void
  showMessage: (ok: boolean, text: string) => void

  // Navigation
  backPath?: string
  backLabel?: string
  title?: string
  showRefresh?: boolean
  showDelete?: boolean

  // Tab
  tab: number
  setTab: (tab: number) => void
}

export default function ContainerDetailBase({
  // Data
  record,
  spec,
  selectedEnvConfig,
  envConfigs,
  envConfigId,
  setEnvConfigId,
  displayedDockerCmd,
  editedDockerCmd,
  setEditedDockerCmd,
  applyingCommand,
  isBuilding,
  followBuildLog,
  setFollowBuildLog,
  buildLog,
  buildLogError,
  buildLogBoxRef,
  logs,
  logsError,
  logsLive,
  logsLoading,
  // Status
  updating,
  deleting,
  confirmDelete,
  acting,
  busy,
  syncing,
  syncError,
  syncSuccess,
  updateMsg,
  // Actions
  verifyContainerStatus,
  handleDelete,
  handleRefresh,
  applyEditedCommand,
  handleOp,
  handleCopyLogs,
  showMessage,
  // Navigation
  backPath,
  backLabel,
  title,
  showRefresh = true,
  showDelete = true,
  // Tab
  tab,
  setTab,
}: ContainerDetailBaseProps) {
  return (
    <Box className="container-detail-page">
      <ContainerHeader
        record={record}
        spec={spec}
        title={title}
        onBack={() => window.history.back()}
        backPath={backPath}
        backLabel={backLabel}
      />

      <ContainerActions
        record={record}
        busy={busy}
        syncing={syncing}
        updating={updating}
        deleting={deleting}
        confirmDelete={confirmDelete}
        acting={acting}
        showRefresh={showRefresh}
        showDelete={showDelete}
        onVerifyStatus={() => { void verifyContainerStatus() }}
        onRestart={() => { void handleOp('restart') }}
        onRefresh={() => { void handleRefresh() }}
        onStart={() => { void handleOp('start') }}
        onStop={() => { void handleOp('stop') }}
        onDelete={() => { void handleDelete() }}
      />

      {updateMsg && <Alert severity={updateMsg.ok ? 'success' : 'error'}>{updateMsg.text}</Alert>}
      <Snackbar open={!!syncError} autoHideDuration={6000} onClose={() => showMessage(false, '')}>
        <Alert severity="error" onClose={() => showMessage(false, '')} sx={{ flexGrow: 1 }}>
          {syncError}
        </Alert>
      </Snackbar>
      <Snackbar open={!!syncSuccess} autoHideDuration={6000} onClose={() => showMessage(true, '')}>
        <Alert severity="success" onClose={() => showMessage(true, '')} sx={{ flexGrow: 1 }}>
          {syncSuccess}
        </Alert>
      </Snackbar>

      <ContainerTabs value={tab} onChange={(_, v) => setTab(v)} />

      {tab === 0 && (
        <Stack spacing={3}>
          {(isBuilding || record?.status === 'error' || (buildLog !== null && buildLog !== '')) && (
            <Card>
              <CardContent>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                  <Typography variant="h6" sx={{ fontWeight: 650, display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Terminal size={18} /> {isBuilding ? 'Сборка образа — живой лог' : 'Лог сборки (последняя попытка)'}
                  </Typography>
                  <label style={{ display: 'flex', alignItems: 'center', gap: 4, cursor: 'pointer' }}>
                    <input
                      type="checkbox"
                      checked={followBuildLog}
                      onChange={(e) => setFollowBuildLog(e.target.checked)}
                    />
                    <Typography variant="caption">Автоскролл</Typography>
                  </label>
                </Box>
                <Box
                  ref={buildLogBoxRef}
                  sx={{
                    bgcolor: '#0b1220',
                    color: '#e2e8f0',
                    fontFamily: 'monospace',
                    fontSize: 12,
                    p: 2,
                    borderRadius: 2,
                    maxHeight: 380,
                    overflowY: 'auto',
                    whiteSpace: 'pre-wrap',
                  }}
                >
                  {buildLog === null ? (buildLogError || (isBuilding ? 'Подключение к логу сборки...' : 'Лог сборки недоступен — возможно, агент его не сохранил')) : buildLog === '' ? (isBuilding ? 'Пока пусто — первые шаги появятся через пару секунд...' : 'Лог пустой') : buildLog}
                </Box>
              </CardContent>
            </Card>
          )}

          <Grid container spacing={3} className="overview-grid">
            <Grid size={{ xs: 12, sm: 4 }}>
              <Card>
                <CardContent>
                  <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: 40, height: 40, borderRadius: 2, bgcolor: 'primary.main', color: 'primary.contrastText' }}>
                      <BoxIcon size={18} />
                    </Box>
                    <Box sx={{ minWidth: 0, flex: 1 }}>
                      <Typography variant="body2" color="text.secondary">Образ</Typography>
                      <Tooltip title={spec?.image || record?.image || ''}>
                        <Typography variant="body1" noWrap sx={{ fontWeight: 600 }}>
                          {spec?.image || record?.image || '—'}
                        </Typography>
                      </Tooltip>
                    </Box>
                  </Stack>
                </CardContent>
              </Card>
            </Grid>
            <Grid size={{ xs: 12, sm: 4 }}>
              <Card>
                <CardContent>
                  <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: 40, height: 40, borderRadius: 2, bgcolor: 'success.main', color: 'success.contrastText' }}>
                      <Server size={18} />
                    </Box>
                    <Box>
                      <Typography variant="body2" color="text.secondary">Узел</Typography>
                      <Typography variant="body1" sx={{ fontWeight: 600 }}>{record?.node_id || '—'}</Typography>
                    </Box>
                  </Stack>
                </CardContent>
              </Card>
            </Grid>
            <Grid size={{ xs: 12, sm: 4 }}>
              <Card>
                <CardContent>
                  <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: 40, height: 40, borderRadius: 2, bgcolor: 'info.main', color: 'info.contrastText' }}>
                      <Activity size={18} />
                    </Box>
                    <Box>
                      <Typography variant="body2" color="text.secondary">Статус</Typography>
                      <Typography variant="body1" sx={{ fontWeight: 600 }}>
                        {record ? `${statusLabel(record.status)} · ${timeAgo(record.updated_at)}` : '—'}
                      </Typography>
                    </Box>
                  </Stack>
                </CardContent>
              </Card>
            </Grid>
          </Grid>

          <PortsList spec={spec} record={record} />
          <VolumesList spec={spec} />
          <PortQuota compact />
        </Stack>
      )}

      {tab === 1 && (
        <Card>
          <CardContent>
            <Typography variant="h6" sx={{ fontWeight: 650, mb: 2, display: 'flex', alignItems: 'center', gap: 1 }}>
              <Terminal size={18} /> Логи приложения
              {logsLive && <Chip label="live" size="small" color="success" variant="outlined" />}
            </Typography>
            {logsLoading && <Typography color="text.secondary">Загрузка логов...</Typography>}
            {logsError && !logsLoading && <Alert severity="warning" sx={{ mb: 1 }}>{logsError}</Alert>}
            {!logsLoading && logs !== null && (
              <Box sx={{ bgcolor: '#0b1220', color: '#e2e8f0', fontFamily: 'monospace', fontSize: 12, p: 2, borderRadius: 2, maxHeight: 380, overflowY: 'auto', whiteSpace: 'pre-wrap' }}>
                {logs.length === 0 && '— нет логов —'}
                {logs.map((line, i) => <div key={i}>{line}</div>)}
              </Box>
            )}
            {!logsLoading && logs === null && !logsError && (
              <Typography color="text.secondary">Контейнер не найден на сервере</Typography>
            )}
            <Button size="small" sx={{ mt: 1 }} startIcon={<Copy size={14} />} onClick={handleCopyLogs}>
              Копировать логи
            </Button>
          </CardContent>
        </Card>
      )}

      {tab === 2 && (
        <Stack spacing={2}>
          <Card>
            <CardContent>
              <DockerCommandEditor
                displayedDockerCmd={displayedDockerCmd}
                editedDockerCmd={editedDockerCmd}
                setEditedDockerCmd={setEditedDockerCmd}
                applyingCommand={applyingCommand}
                onApply={applyEditedCommand}
                onCopy={() => copyText(displayedDockerCmd)}
              />
              <EnvConfigSelector
                configs={envConfigs}
                selectedEnvConfig={selectedEnvConfig}
                envConfigId={envConfigId}
                onEnvConfigChange={setEnvConfigId}
                spec={spec}
              />
              <PortQuota compact />
            </CardContent>
          </Card>
        </Stack>
      )}
    </Box>
  )
}