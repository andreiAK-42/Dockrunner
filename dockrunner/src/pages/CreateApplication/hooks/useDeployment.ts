import { useState, useCallback } from 'react'
import { createDeploymentRequest } from '../../../api/deploymentRequests'
import { fetchQuotas } from '../../../api/quotas'
import type { MappedPort, MappedVolume } from '../../../utils/docker'
import type { PortInfo } from '../../../hooks/usePorts'

export interface UseDeploymentOptions {
  appName: string
  repoUrl: string
  effectiveNodeId: string
  buildMode: 'dockerfile' | 'compose'
  selectedServices: string[]
  ports: { host: string; container: string }[]
  volumes: { hostPath: string; containerPath: string }[]
  envRows: { key: string; value: string }[]
  cpuCores: number
  memoryGB: number
  diskGB: number
  durationHours: number
  portsRange: { min: number; max: number }
  busyPorts: PortInfo[]
  buildMappedPorts: () => MappedPort[]
  buildMappedVolumes: () => MappedVolume[]
  buildEnvMap: () => Record<string, string>
  setFormError: React.Dispatch<React.SetStateAction<string>>
}

export interface UseDeploymentReturn {
  deploying: boolean
  requestSubmitted: boolean
  requestId: string | null
  formError: string
  setFormError: React.Dispatch<React.SetStateAction<string>>
  handleDeploy: () => Promise<void>
}

/**
 * Hook for handling deployment logic
 */
export function useDeployment(options: UseDeploymentOptions): UseDeploymentReturn {
  const {
    appName, repoUrl, effectiveNodeId, buildMode, selectedServices,
    ports, volumes, envRows, cpuCores, memoryGB, diskGB, durationHours,
    portsRange, busyPorts, buildMappedPorts, buildMappedVolumes, buildEnvMap, setFormError,
  } = options

  const [deploying, setDeploying] = useState(false)
  const [requestSubmitted, setRequestSubmitted] = useState(false)
  const [requestId, setRequestId] = useState<string | null>(null)
  const [formError, setFormErrorState] = useState('')

  const setFormErrorWrapper = useCallback((error: string) => {
    setFormErrorState(error)
    setFormError(error)
  }, [setFormError])

  const handleDeploy = useCallback(async () => {
    if (buildMode === 'compose' && selectedServices.length === 0) {
      setFormErrorWrapper('Выберите хотя бы один сервис Docker Compose')
      return
    }

    let mappedPorts: MappedPort[]
    let mappedVolumes: MappedVolume[]
    try {
      mappedPorts = buildMappedPorts()
      mappedVolumes = buildMappedVolumes()
    } catch (validationError) {
      if (validationError instanceof Error) {
        setFormErrorWrapper(validationError.message)
      } else {
        setFormErrorWrapper('Проверьте порты и тома')
      }
      return
    }
    const env = buildEnvMap()

    setFormErrorWrapper('')
    setDeploying(true)

    try {
      const quotas = await fetchQuotas().catch(() => null)
      if (quotas) {
        const freePorts = quotas.data.ports_limit - quotas.data.ports_used
        if (mappedPorts.length > freePorts) {
          throw new Error(`Превышен лимит портов: свободно ${freePorts}`)
        }
      }

      const res = await createDeploymentRequest({
        app_name: appName.trim(),
        repo_url: repoUrl.trim(),
        container_name: appName.trim(),
        node_id: effectiveNodeId,
        ports: mappedPorts,
        volumes: mappedVolumes,
        env,
        cpu_cores: cpuCores,
        memory_gb: memoryGB,
        disk_gb: diskGB,
        duration_hours: durationHours,
        build_mode: buildMode,
        compose_services: buildMode === 'compose' ? selectedServices : undefined,
      })

      setRequestSubmitted(true)
      setRequestId(res.data.deployment_id)
      setFormErrorWrapper('')
    } catch (deployError) {
      if (deployError instanceof Error) {
        setFormErrorWrapper(deployError.message)
      } else {
        setFormErrorWrapper('Отправка заявки не удалась')
      }
    } finally {
      setDeploying(false)
    }
  }, [
    appName, repoUrl, effectiveNodeId, buildMode, selectedServices,
    buildMappedPorts, buildMappedVolumes, buildEnvMap,
    cpuCores, memoryGB, diskGB, durationHours, setFormErrorWrapper,
  ])

  return {
    deploying,
    requestSubmitted,
    requestId,
    formError,
    setFormError: setFormErrorWrapper,
    handleDeploy,
  }
}