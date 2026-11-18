import { apiRequest } from './client'

export interface FileEntry {
  name: string
  is_dir: boolean
  size: number
}

function ownerQuery(owner?: string) {
  if (owner) {
    return `&owner=${encodeURIComponent(owner)}`
  }
  return ''
}

export function listFiles(node: string, path: string, owner?: string) {
  return apiRequest<{ data: FileEntry[] }>(`/api/v1/files?node=${encodeURIComponent(node)}&path=${encodeURIComponent(path)}${ownerQuery(owner)}`)
}

export function mkdir(node: string, path: string, owner?: string) {
  return apiRequest('/api/v1/files/mkdir', {
    method: 'POST',
    body: { node_id: node, path, owner },
  })
}

export async function uploadFile(node: string, path: string, file: File, owner?: string) {
  const form = new FormData()
  form.append('node_id', node)
  form.append('path', path)
  if (owner) {
    form.append('owner', owner)
  }
  form.append('file', file)
  await apiRequest('/api/v1/files/upload', { method: 'POST', form })
}

export function downloadFile(node: string, path: string, owner?: string) {
  return apiRequest<Blob>(
    `/api/v1/files/download?node=${encodeURIComponent(node)}&path=${encodeURIComponent(path)}${ownerQuery(owner)}`,
    { blob: true },
  )
}

export function deleteFile(node: string, path: string, owner?: string) {
  return apiRequest(`/api/v1/files?node=${encodeURIComponent(node)}&path=${encodeURIComponent(path)}${ownerQuery(owner)}`, {
    method: 'DELETE',
  })
}
