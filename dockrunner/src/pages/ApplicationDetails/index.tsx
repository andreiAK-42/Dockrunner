import { useParams, useNavigate } from 'react-router-dom'
import { useState } from 'react'
import { useEnvConfigs } from '../../hooks/useEnvConfigs'
import { useAppSelector } from '../../store/hooks'
import ContainerDetailBase from '../../components/containers/ContainerDetailBase'
import { useContainerDetail } from '../../hooks/useContainerDetail'
import type { MineContainer } from '../../api/containers'

export default function ApplicationDetails() {
  const { id } = useParams()
  const navigate = useNavigate()
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

  return (
    <ContainerDetailBase
      {...containerDetail}
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