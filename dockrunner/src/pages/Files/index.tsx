import { useEffect, useRef, useState } from 'react'
import { Box, Button, Card, CardContent, Stack, Typography, TextField, Dialog, DialogTitle, DialogContent, DialogActions, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, FormControl, InputLabel, Select, MenuItem, Alert, IconButton } from '@mui/material'
import { FolderPlus, Upload, Download, Folder, FileText, Trash2 } from 'lucide-react'
import { listFiles, mkdir, uploadFile, downloadFile, deleteFile } from '../../api/files'
import type { FileEntry } from '../../api/files'
import { fetchAdminUsers } from '../../api/admin'
import type { AdminUser } from '../../api/admin'
import { useAgents } from '../../hooks/useAgents'
import { useAuth } from '../../auth/AuthContext'
import styles from './Files.module.scss'

function joinPath(base: string, name: string) {
  if (base === '/') {
    return `/${name}`
  }
  return `${base}/${name}`
}

function parentPath(path: string) {
  if (path === '/') {
    return '/'
  }
  const withoutLast = path.split('/').slice(0, -1).join('/')
  if (!withoutLast) {
    return '/'
  }
  return withoutLast
}

function formatSizeKb(sizeBytes: number) {
  return `${(sizeBytes / 1024).toFixed(1)} КБ`
}

export default function Files() {
  const { nodes } = useAgents()
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'
  const online = nodes.filter((node) => node.status === 'online')
  const [selectedNode, setSelectedNode] = useState('')
  const [path, setPath] = useState('/')
  const [entries, setEntries] = useState<FileEntry[]>([])
  const [error, setError] = useState('')
  const [mkdirOpen, setMkdirOpen] = useState(false)
  const [dirName, setDirName] = useState('')
  const [busy, setBusy] = useState(false)
  const [allUsers, setAllUsers] = useState<AdminUser[]>([])
  const [ownerLogin, setOwnerLogin] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  let activeNode = selectedNode
  if (!activeNode && online.length > 0) {
    activeNode = online[0].id
  }
  const activeOwner = isAdmin && ownerLogin ? ownerLogin : undefined

  useEffect(() => {
    if (!isAdmin) {
      return
    }
    fetchAdminUsers()
      .then((loadedUsers) => setAllUsers(loadedUsers))
      .catch(() => setAllUsers([]))
  }, [isAdmin])

  const reload = () => {
    if (!activeNode) {
      return
    }
    const loadedNode = activeNode
    const loadedPath = path
    const loadedOwner = activeOwner
    listFiles(loadedNode, loadedPath, loadedOwner)
      .then((res) => setEntries(res.data))
      .catch(() => setError('Не удалось загрузить папку'))
  }

  useEffect(() => {
    if (!activeNode) {
      return
    }
    const loadedNode = activeNode
    const loadedPath = path
    const loadedOwner = activeOwner
    listFiles(loadedNode, loadedPath, loadedOwner)
      .then((res) => {
        setEntries(res.data)
        setError('')
      })
      .catch(() => setError('Не удалось загрузить папку'))
  }, [activeNode, path, activeOwner])

  const openDir = (name: string) => {
    setPath(joinPath(path, name))
  }

  const goUp = () => {
    setPath(parentPath(path))
  }

  const handleSelectNode = (nodeId: string) => {
    setSelectedNode(nodeId)
    setPath('/')
  }

  const handleSelectOwner = (login: string) => {
    setOwnerLogin(login)
    setPath('/')
  }

  const handleMkdir = () => {
    const trimmedName = dirName.trim()
    if (!trimmedName || !activeNode) {
      return
    }
    setBusy(true)
    const newDirPath = joinPath(path, trimmedName)
    const targetNode = activeNode
    mkdir(targetNode, newDirPath, activeOwner)
      .then(() => {
        setMkdirOpen(false)
        setDirName('')
        reload()
      })
      .catch(() => setError('Не удалось создать папку'))
      .finally(() => setBusy(false))
  }

  const handleUpload = (file: File) => {
    if (!activeNode) {
      return
    }
    const target = joinPath(path, file.name)
    const targetNode = activeNode
    uploadFile(targetNode, target, file, activeOwner)
      .then(reload)
      .catch(() => setError('Не удалось загрузить файл'))
  }

  const handleDownload = (name: string) => {
    if (!activeNode) {
      return
    }
    const target = joinPath(path, name)
    const targetNode = activeNode
    downloadFile(targetNode, target, activeOwner)
      .then((blob) => {
        const url = URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = url
        link.download = name
        link.click()
        URL.revokeObjectURL(url)
      })
      .catch(() => setError('Не удалось скачать файл'))
  }

  const handleDelete = (name: string, isDir: boolean) => {
    if (!activeNode) {
      return
    }
    const target = joinPath(path, name)
    const itemLabel = isDir ? 'папку со всем содержимым' : 'файл'
    const confirmed = window.confirm(`Удалить ${itemLabel} «${name}»?`)
    if (!confirmed) {
      return
    }
    const targetNode = activeNode
    deleteFile(targetNode, target, activeOwner)
      .then(reload)
      .catch(() => setError('Не удалось удалить'))
  }

  const handleFileInput = (event: React.ChangeEvent<HTMLInputElement>) => {
    const selectedFile = event.target.files?.[0]
    if (selectedFile) {
      handleUpload(selectedFile)
    }
    event.target.value = ''
  }

  return (
    <Box className={styles.page}>
      <Box className={styles.header}>
        <Box>
          <Typography variant="h4" className={styles.header__title}>Файлы</Typography>
        </Box>
      </Box>

      {error && <Alert severity="error">{error}</Alert>}
      {online.length === 0 && (
        <Alert severity="warning">Невозможно установить подключение с серверами.</Alert>
      )}

      <Card>
        <CardContent>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ mb: 2 }}>
            <FormControl sx={{ minWidth: 220 }}>
              <InputLabel>Сервер</InputLabel>
              <Select value={activeNode} label="Сервер" onChange={(e) => handleSelectNode(e.target.value)}>
                {online.map((worker) => (
                  <MenuItem key={worker.id} value={worker.id}>{worker.name} ({worker.city})</MenuItem>
                ))}
              </Select>
            </FormControl>
            {isAdmin && (
              <FormControl sx={{ minWidth: 220 }}>
                <InputLabel>Пользователь</InputLabel>
                <Select value={ownerLogin} label="Пользователь" onChange={(e) => handleSelectOwner(e.target.value)}>
                  <MenuItem value=""><em>— Мой каталог —</em></MenuItem>
                  {allUsers.map((u) => (
                    <MenuItem key={u.id} value={u.login}>{u.login} ({u.role})</MenuItem>
                  ))}
                </Select>
              </FormControl>
            )}
            <TextField label="Путь" value={path} onChange={(e) => setPath(e.target.value || '/')} sx={{ flex: 1 }} />
            <Stack direction="row" spacing={1}>
              <Button variant="outlined" startIcon={<FolderPlus size={16} />} onClick={() => setMkdirOpen(true)}>Папка</Button>
              <Button variant="contained" startIcon={<Upload size={16} />} onClick={() => fileRef.current?.click()}>Загрузить</Button>
            </Stack>
          </Stack>
          <input ref={fileRef} type="file" hidden onChange={handleFileInput} />

          <TableContainer>
            <Table size="small">
              <TableHead><TableRow><TableCell>Имя</TableCell><TableCell>Размер</TableCell><TableCell>Действия</TableCell></TableRow></TableHead>
              <TableBody>
                {path !== '/' && (
                  <TableRow hover sx={{ cursor: 'pointer' }} onClick={goUp}>
                    <TableCell colSpan={3}><Typography variant="body2" color="text.secondary">.. (наверх)</Typography></TableCell>
                  </TableRow>
                )}
                {entries.map((entry) => (
                  <TableRow key={entry.name} hover sx={{ cursor: entry.is_dir ? 'pointer' : 'default' }} onClick={() => entry.is_dir && openDir(entry.name)}>
                    <TableCell>
                      <Stack direction="row" spacing={1} alignItems="center">
                        {entry.is_dir ? <Folder size={16} /> : <FileText size={16} />}
                        <Typography variant="body2">{entry.name}</Typography>
                      </Stack>
                    </TableCell>
                    <TableCell>{entry.is_dir ? '—' : formatSizeKb(entry.size)}</TableCell>
                    <TableCell>
                      <Stack direction="row" spacing={1}>
                        {!entry.is_dir && (
                          <Button size="small" startIcon={<Download size={14} />} onClick={(e) => { e.stopPropagation(); handleDownload(entry.name) }}>Скачать</Button>
                        )}
                        <IconButton size="small" color="error" onClick={(e) => { e.stopPropagation(); handleDelete(entry.name, entry.is_dir) }}><Trash2 size={16} /></IconButton>
                      </Stack>
                    </TableCell>
                  </TableRow>
                ))}
                {entries.length === 0 && <TableRow><TableCell colSpan={3} sx={{ textAlign: 'center', py: 3, color: 'text.secondary' }}>Папка пуста</TableCell></TableRow>}
              </TableBody>
            </Table>
          </TableContainer>
        </CardContent>
      </Card>

      <Dialog open={mkdirOpen} onClose={() => setMkdirOpen(false)} fullWidth maxWidth="xs">
        <DialogTitle>Новая папка</DialogTitle>
        <DialogContent>
          <TextField label="Название" value={dirName} onChange={(e) => setDirName(e.target.value)} placeholder="data" autoFocus fullWidth sx={{ mt: 1 }} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setMkdirOpen(false)}>Отмена</Button>
          <Button variant="contained" disabled={!dirName.trim() || busy} onClick={handleMkdir}>Создать</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
