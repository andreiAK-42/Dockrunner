export type ContainerStatus = 'running' | 'stopped' | 'restarting' | 'created' | 'error'

export interface ContainerPort {
  hostPort: number
  containerPort: number
  protocol: 'tcp' | 'udp'
}

export interface ContainerVolume {
  id: string
  name: string
  mountPath: string
  sizeGB: number
  readOnly?: boolean
}

export interface ContainerEnv {
  key: string
  value: string
}

export interface Container {
  id: string
  name: string
  appId: string
  image: string
  status: ContainerStatus
  nodeId: string
  ports: ContainerPort[]
  volumes: ContainerVolume[]
  env: ContainerEnv[]
  envConfigId?: string
  command?: string
  restartPolicy: 'no' | 'always' | 'on-failure' | 'unless-stopped'
  createdAt: string
  logs: string[]
}
