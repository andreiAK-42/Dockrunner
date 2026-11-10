// Docker-related utilities for port/volume/env mapping

import type { PortInfo } from '../hooks/usePorts'
import type { DetectedPort } from '../api/containers'

export interface VolumeRow {
  hostPath: string
  containerPath: string
}

export interface EnvRow {
  key: string
  value: string
}

export interface MappedVolume {
  name?: string
  host_path?: string
  container_path: string
}

export interface MappedPort {
  host_port: number
  container_port: number
}

/**
 * Get human-readable label for port source
 */
export function portSourceLabel(port: DetectedPort): string {
  if (port.source === 'dockerfile') {
    return 'Dockerfile EXPOSE'
  }
  if (port.service) {
    return `docker-compose: ${port.service}`
  }
  return 'docker-compose'
}

/**
 * Build mapped volumes from form rows
 */
export function buildMappedVolumes(volumeRows: VolumeRow[]): MappedVolume[] {
  const mappedVolumes: MappedVolume[] = []

  for (const row of volumeRows) {
    const containerPath = row.containerPath.trim()
    const hostPath = row.hostPath.trim()

    const bothEmpty = !containerPath && !hostPath
    if (bothEmpty) {
      continue
    }

    if (!containerPath) {
      throw new Error('У каждого тома должен быть путь в контейнере')
    }

    const volume: MappedVolume = { container_path: containerPath }

    if (hostPath) {
      const isPath = hostPath.includes('/') || hostPath.includes('\\') || hostPath.startsWith('.')
      if (isPath) {
        volume.host_path = hostPath
      } else {
        volume.name = hostPath
      }
    }

    mappedVolumes.push(volume)
  }

  return mappedVolumes
}

/**
 * Build environment map from form rows
 */
export function buildEnvMap(envRows: EnvRow[]): Record<string, string> {
  const env: Record<string, string> = {}

  for (const row of envRows) {
    const key = row.key.trim()
    if (key) {
      env[key] = row.value
    }
  }

  return env
}