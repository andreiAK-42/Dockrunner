import { useParams, useNavigate } from 'react-router-dom'
import { useState } from 'react'
import { useEnvConfigs } from '../../hooks/useEnvConfigs'
import { useAppDispatch, useAppSelector } from '../../store/hooks'
import { containerStatusPatched, fetchMineContainers } from '../../store/slices/containersSlice'
import { fetchContainerSpec } from '../../api/containers'
import ContainerDetailBase from '../../components/containers/ContainerDetailBase'
import { useContainerDetail } from '../../hooks/useContainerDetail'
import type { MineContainer } from '../../api/containers'

export default function ApplicationDetails() {
  const { id } = useParams()
  const navigate = useNavigate()
  const dispatch = useAppDispatch()
  const [tab, setTab] = useState(0)
  const { configs } = useEnvConfigs()

  const record = useAppSelector(
    (state) => state.containers.mine.find((container) => container.app_name === id) || null,
  )

  const containerDetail = useContainerDetail({
    id: id || '',
    findContainer: (containers) => containers.find((c: MineContainer) => c.app_name === id) || null,
    onDeleted: () => navigate('/applications'),
  })

  const selectedEnv = configs.find((config) => config.id === containerDetail.envConfigId)

  // Override verifyContainerStatus to dispatch the status patch
  const originalVerifyStatus = containerDetail.verifyContainerStatus
  const verifyContainerStatus = async () => {
    await originalVerifyStatus()
    // After verification, we need to sync with redux
    try {
      const containers = await dispatch(fetchMineContainers()).unwrap()
      const currentRecord = containers.find((c) => c.app_name === id) || null
      if (currentRecord && record) {
        const specRes = await fetchContainerSpec(currentRecord.id)
        const actualStatus = specRes.data.status
        if (actualStatus !== currentRecord.status) {
          dispatch(containerStatusPatched({ id: currentRecord.id, status: actualStatus }))
        }
      }
    } catch {
      // ignore
    }
  }

  return (
    <ContainerDetailBase
      {...containerDetail}
      verifyContainerStatus={verifyContainerStatus}
      record={record}
      spec={containerDetail.spec}
      backPath="/applications"
      backLabel="Назад к приложениям"
      title={record?.app_name}
      showRefresh={true}
      showDelete={true}
      envConfigs={configs}
      selectedEnvConfig={selectedEnv}
      envConfigId={containerDetail.envConfigId}
      setEnvConfigId={containerDetail.setEnvConfigId}
      tab={tab}
      setTab={setTab}
    />
  )
}