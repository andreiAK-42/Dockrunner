import { useParams } from 'react-router-dom'
import { useEnvConfigs } from '../../hooks/useEnvConfigs'
import { useAppSelector } from '../../store/hooks'
import ContainerDetailBase from '../../components/containers/ContainerDetailBase'
import { useContainerDetail } from '../../hooks/useContainerDetail'
import type { MineContainer } from '../../api/containers'

export default function ContainerDetails() {
  const { id, containerId } = useParams()
  const realId = containerId || id || ''
  const { configs } = useEnvConfigs()

  const record = useAppSelector(
    (state) => state.containers.mine.find((c: MineContainer) => c.id === realId) || null,
  )

  const containerDetail = useContainerDetail({
    id: realId,
    findContainer: (containers) => containers.find((c: MineContainer) => c.id === realId) || null,
    onDeleted: () => {}, // navigation handled by hook
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
      showRefresh={false}
      showDelete={true}
      envConfigs={configs}
      selectedEnvConfig={selectedEnv}
      envConfigId={containerDetail.envConfigId}
      setEnvConfigId={containerDetail.setEnvConfigId}
      tab={0}
      setTab={() => {}}
    />
  )
}