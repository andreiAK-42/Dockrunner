import { useState } from 'react'
import { Box, Button, Card, CardContent, Stack, Typography, Dialog, DialogTitle, DialogContent, DialogActions, TextField, IconButton, Chip } from '@mui/material'
import { Plus, Trash2, Edit2, Copy, FileCode2, AlertCircle } from 'lucide-react'
import { useEnvConfigs } from '../../hooks/useEnvConfigs'
import { copyText } from '../../utils/clipboard'
import { toDockerEnvArgs } from '../../types/envConfig'
import type { EnvVar } from '../../types/envConfig'
import styles from './EnvConfigs.module.scss'

export default function EnvConfigs() {
  const { configs, loading, canCreate, remaining, create, update, remove } = useEnvConfigs()
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [desc, setDesc] = useState('')
  const [vars, setVars] = useState<EnvVar[]>([{ key: '', value: '' }])
  const [error, setError] = useState('')

  const reset = () => {
    setName('')
    setDesc('')
    setVars([{ key: '', value: '' }])
    setEditingId(null)
    setError('')
  }

  const handleOpenCreate = () => {
    if (!canCreate) {
      setError('Достигнут лимит 10 конфигов')
      return
    }
    reset()
    setOpen(true)
  }

  const handleOpenEdit = (id: string) => {
    const foundConfig = configs.find((config) => config.id === id)
    if (!foundConfig) {
      return
    }
    setEditingId(id)
    setName(foundConfig.name)
    setDesc(foundConfig.description || '')

    if (foundConfig.vars.length > 0) {
      setVars(foundConfig.vars)
    } else {
      setVars([{ key: '', value: '' }])
    }

    setOpen(true)
  }

  const handleSave = () => {
    const trimmedName = name.trim()
    if (!trimmedName) {
      setError('Введите название')
      return
    }

    const filledVars = vars
      .filter((envVar) => envVar.key.trim())
      .map((envVar) => ({ key: envVar.key.trim(), value: envVar.value }))

    if (filledVars.length === 0) {
      setError('Добавьте хотя бы одну переменную')
      return
    }

    const keys = filledVars.map((envVar) => envVar.key)
    const hasDuplicates = new Set(keys).size !== keys.length
    if (hasDuplicates) {
      setError('Ключи не должны дублироваться')
      return
    }

    const trimmedDesc = desc.trim()

    let save: Promise<unknown>
    if (editingId) {
      save = update(editingId, { name: trimmedName, description: trimmedDesc, vars: filledVars })
    } else {
      save = create(trimmedName, trimmedDesc, filledVars)
    }

    save
      .then(() => {
        setOpen(false)
        reset()
      })
      .catch((saveError) => {
        if (saveError instanceof Error) {
          setError(saveError.message)
        } else {
          setError('Не удалось сохранить')
        }
      })
  }

  const addVarRow = () => {
    setVars([...vars, { key: '', value: '' }])
  }

  const updateVar = (index: number, patch: Partial<EnvVar>) => {
    const nextVars = vars.map((envVar, varIndex) => {
      if (varIndex === index) {
        return { ...envVar, ...patch }
      }
      return envVar
    })
    setVars(nextVars)
  }

  const removeVar = (index: number) => {
    setVars(vars.filter((_, varIndex) => varIndex !== index))
  }

  const copyDocker = (id: string) => {
    const foundConfig = configs.find((config) => config.id === id)
    if (!foundConfig) {
      return
    }
    copyText(toDockerEnvArgs(foundConfig))
  }

  return (
    <div className={styles.page}>
      <Box className={styles.header}>
        <Box>
          <Typography variant="h4" className={styles.header__title}>Env конфигурации</Typography>
          <Typography color="text.secondary" variant="body2">Сохраняйте наборы переменных окружения и применяйте их при запуске контейнера через <code>docker run --env</code>. До 10 конфигов на пользователя.</Typography>
        </Box>
        <Button variant="contained" startIcon={<Plus size={18} />} onClick={handleOpenCreate} disabled={!canCreate}>Создать</Button>
      </Box>

      <Card>
        <CardContent>
          <Stack direction="row" justifyContent="space-between" alignItems="center">
            <Typography variant="body2" sx={{ fontWeight: 600 }}>Использовано {configs.length} / 10</Typography>
            <Chip label={`Осталось ${remaining}`} color={remaining === 0 ? 'error' : 'success'} size="small" />
          </Stack>
          <Box sx={{ mt: 1.5, height: 8, borderRadius: 999, bgcolor: 'divider', overflow: 'hidden' }}>
            <Box sx={{ width: `${configs.length * 10}%`, height: '100%', bgcolor: configs.length >= 10 ? 'error.main' : 'primary.main', transition: 'width .3s' }} />
          </Box>
          {!canCreate && <Stack direction="row" gap={1} alignItems="center" sx={{ mt: 1.5, color: 'warning.main' }}><AlertCircle size={16} /><Typography variant="caption">Лимит исчерпан — удалите старый конфиг чтобы создать новый.</Typography></Stack>}
        </CardContent>
      </Card>

      {loading ? (
        <Card><CardContent sx={{ textAlign: 'center', py: 4 }}><Typography color="text.secondary">Загрузка...</Typography></CardContent></Card>
      ) : configs.length === 0 ? (
        <Card><CardContent className={styles.empty}><FileCode2 size={32} style={{ margin: '0 auto 8px', opacity: .5 }} /><Typography>Нет конфигов — создайте первый</Typography></CardContent></Card>
      ) : (
        <Box className={styles.grid}>
          {configs.map((config) => (
            <Box key={config.id} className={styles.card}>
              <Box className={styles.card__header}>
                <Box>
                  <Typography className={styles.card__name}>{config.name}</Typography>
                  {config.description && <Typography className={styles.card__desc}>{config.description}</Typography>}
                  <Typography variant="caption" color="text.secondary">{new Date(config.updatedAt).toLocaleString()} • {config.vars.length} перем.</Typography>
                </Box>
                <Chip size="small" label={`${config.vars.length} перем.`} />
              </Box>

              <Box className={styles.card__vars}>
                {config.vars.map((envVar) => (
                  <Box key={envVar.key} className={styles.card__var}>
                    <span className={styles.card__varKey}>{envVar.key}=</span>
                    <span className={styles.card__varVal}>{envVar.value || '""'}</span>
                  </Box>
                ))}
              </Box>

              <Box className={styles.card__preview}>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
                  <span style={{ opacity: .7 }}>превью docker run</span>
                  <IconButton size="small" onClick={() => copyDocker(config.id)} sx={{ p: 0.3, color: '#e2e8f0' }}><Copy size={12} /></IconButton>
                </Box>
                {toDockerEnvArgs(config) || '--env ...'}
              </Box>

              <Box className={styles.card__actions}>
                <Button size="small" startIcon={<Edit2 size={14} />} onClick={() => handleOpenEdit(config.id)}>Изменить</Button>
                <Button size="small" color="error" startIcon={<Trash2 size={14} />} onClick={() => remove(config.id)}>Удалить</Button>
              </Box>
            </Box>
          ))}
        </Box>
      )}

      <Dialog open={open} onClose={() => setOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>{editingId ? 'Редактировать конфиг' : 'Новый env-конфиг'}</DialogTitle>
        <DialogContent>
          <Stack className={styles.dialogContent}>
            {error && <Box sx={{ px: 1.5, py: 1, borderRadius: 1, bgcolor: 'error.light', color: 'error.contrastText', fontSize: 13 }}>{error}</Box>}
            <TextField label="Название" value={name} onChange={e => setName(e.target.value)} placeholder="production" required />
            <TextField label="Описание" value={desc} onChange={e => setDesc(e.target.value)} placeholder="Переменные для прода" />
            <Typography variant="subtitle2" sx={{ fontWeight: 600, mt: 1 }}>Переменные окружения</Typography>
            {vars.map((envVar, varIndex) => (
              <Box key={varIndex} className={styles.varRow}>
                <TextField size="small" label="Ключ" value={envVar.key} onChange={e => updateVar(varIndex, { key: e.target.value })} placeholder="NODE_ENV" sx={{ flex: 1 }} />
                <TextField size="small" label="Значение" value={envVar.value} onChange={e => updateVar(varIndex, { value: e.target.value })} placeholder="production" sx={{ flex: 1.6 }} />
                <IconButton size="small" color="error" onClick={() => removeVar(varIndex)} disabled={vars.length === 1}><Trash2 size={16} /></IconButton>
              </Box>
            ))}
            <Button variant="outlined" size="small" startIcon={<Plus size={14} />} onClick={addVarRow}>Добавить переменную</Button>
            {!canCreate && !editingId && <Typography variant="caption" color="error">Лимит 10 достигнут</Typography>}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpen(false)}>Отмена</Button>
          <Button variant="contained" onClick={handleSave}>{editingId ? 'Сохранить' : 'Создать'}</Button>
        </DialogActions>
      </Dialog>
    </div>
  )
}
