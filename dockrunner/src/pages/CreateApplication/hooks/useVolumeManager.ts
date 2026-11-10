import { useState, useCallback } from 'react'
import { buildMappedVolumes, type VolumeRow, type MappedVolume } from '../../../utils/docker'

export interface UseVolumeManagerReturn {
  volumes: VolumeRow[]
  setVolumes: React.Dispatch<React.SetStateAction<VolumeRow[]>>
  updateVolume: (index: number, patch: Partial<VolumeRow>) => void
  removeVolume: (index: number) => void
  buildMappedVolumes: () => MappedVolume[]
}

/**
 * Hook for managing volume form state
 */
export function useVolumeManager(): UseVolumeManagerReturn {
  const [volumes, setVolumes] = useState<VolumeRow[]>([{ hostPath: '', containerPath: '' }])

  const updateVolume = useCallback((index: number, patch: Partial<VolumeRow>) => {
    setVolumes((prev) => prev.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  }, [])

  const removeVolume = useCallback((index: number) => {
    setVolumes((prev) => prev.filter((_, i) => i !== index))
  }, [])

  const buildMappedVolumesResult = useCallback((): MappedVolume[] => {
    return buildMappedVolumes(volumes)
  }, [volumes])

  return {
    volumes,
    setVolumes,
    updateVolume,
    removeVolume,
    buildMappedVolumes: buildMappedVolumesResult,
  }
}