export interface EnvVar {
  key: string
  value: string
}

export interface EnvConfig {
  id: string
  name: string
  description?: string
  vars: EnvVar[]
  createdAt: string
  updatedAt: string
}

export const ENV_CONFIG_LIMIT = 10

export function toDockerEnvArgs(config: EnvConfig): string {
  const args = config.vars.map((envVar) => `--env ${envVar.key}="${envVar.value}"`)
  return args.join(' ')
}

export function toEnvFileContent(config: EnvConfig): string {
  const lines = config.vars.map((envVar) => `${envVar.key}=${envVar.value}`)
  return lines.join('\n')
}
