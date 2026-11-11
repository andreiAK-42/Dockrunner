import { Box, TextField, Button, Alert, Stack, Divider, Typography } from '@mui/material'
import { Copy } from 'lucide-react'

interface DockerCommandEditorProps {
  displayedDockerCmd: string
  editedDockerCmd: string | null
  setEditedDockerCmd: (cmd: string | null) => void
  applyingCommand: boolean
  onApply: () => void
  onCopy: () => void
}

export default function DockerCommandEditor({
  displayedDockerCmd,
  editedDockerCmd,
  setEditedDockerCmd,
  applyingCommand,
  onApply,
  onCopy,
}: DockerCommandEditorProps) {
  return (
    <Box>
      <Typography variant="h6" sx={{ fontWeight: 650, mb: 1, display: 'flex', alignItems: 'center', gap: 1 }}>
        Параметры запуска
      </Typography>
      <Stack spacing={1.5}>
        <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', alignItems: 'center' }}>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>{displayedDockerCmd ? 'Редактируемая команда' : 'Команда не сформирована'}</Typography>
        </Box>
        <TextField
          fullWidth
          multiline
          minRows={3}
          maxRows={10}
          label="Команда запуска (можно редактировать)"
          value={displayedDockerCmd}
          onChange={(e) => setEditedDockerCmd(e.target.value)}
          placeholder="Команда не сформирована: проверьте образ и имя контейнера"
          sx={{ '& textarea': { fontFamily: 'monospace', fontSize: 12 } }}
        />
        {!displayedDockerCmd && (
          <Alert severity="warning">
            Команда не сформирована. Проверьте, что API агента возвращает образ, имя и параметры контейнера.
          </Alert>
        )}
        <Alert severity="info">
          Можно изменить строку и скопировать её. Это не изменяет уже созданный контейнер: чтобы применить параметры, его необходимо пересоздать.
        </Alert>
        <Stack direction="row" spacing={1}>
          <Button size="small" startIcon={<Copy size={14} />} onClick={onCopy} disabled={!displayedDockerCmd.trim()}>
            Копировать команду
          </Button>
          <Button size="small" onClick={() => setEditedDockerCmd(null)} disabled={editedDockerCmd === null}>
            Сбросить изменения
          </Button>
          <Button
            size="small"
            variant="contained"
            onClick={onApply}
            disabled={applyingCommand || !displayedDockerCmd.trim()}
          >
            {applyingCommand ? 'Пересоздание…' : 'Применить и пересоздать'}
          </Button>
        </Stack>
        <Divider />
      </Stack>
    </Box>
  )
}