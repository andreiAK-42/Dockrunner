// Docker command building utilities - single source of truth

import type { ContainerSpec } from '../api/containers'

/**
 * Shell-escape a value for safe use in docker commands
 */
export function shellArg(value: string): string {
  if (/^[A-Za-z0-9_./:@%+=,-]+$/.test(value)) return value
  return `'${value.replace(/'/g, `"'"'`)}'`
}

/**
 * Build environment args from container spec
 */
export function specEnvArgs(spec: ContainerSpec | null): string {
  if (!spec) return ''
  return Object.entries(spec.env)
    .map(([key, value]) => `--env ${shellArg(`${key}=${value}`)}`)
    .join(' ')
}

/**
 * Format a volume mount for docker command
 */
export function formatVolumeMount(volume: { name: string; host_path: string; container_path: string }): string {
  const source = volume.name || volume.host_path
  return `-v ${shellArg(`${source}:${volume.container_path}`)}`
}

/**
 * Build docker run command from container spec and environment args
 */
export function buildDockerRun(spec: ContainerSpec | null, envArgs: string): string {
  if (!spec) return ''
  const args = ['docker run -d', `--name ${shellArg(spec.name)}`]

  for (const port of spec.ports) {
    args.push(`-p ${port.host}:${port.container}`)
  }

  args.push(...spec.volumes.map(formatVolumeMount), envArgs)

  if (spec.restart_policy) args.push(`--restart ${shellArg(spec.restart_policy)}`)
  if (spec.network) args.push(`--network ${shellArg(spec.network)}`)
  if (spec.entrypoint?.length) args.push(`--entrypoint ${shellArg(spec.entrypoint[0])}`)
  if (spec.working_dir) args.push(`-w ${shellArg(spec.working_dir)}`)
  if (spec.user) args.push(`-u ${shellArg(spec.user)}`)

  args.push(shellArg(spec.image))

  if (spec.cmd?.length || (spec.entrypoint && spec.entrypoint.length > 1)) {
    const cmdArgs = [...(spec.entrypoint?.slice(1) || []), ...(spec.cmd || [])]
    args.push(cmdArgs.map(shellArg).join(' '))
  }

  return args.filter(Boolean).join(' ').trim()
}

/**
 * Build docker run command from env config vars (for preview)
 */
export function buildDockerRunFromEnvConfig(
  spec: ContainerSpec | null,
  envConfigVars: { key: string; value: string }[]
): string {
  if (!spec) return ''
  const envArgs = envConfigVars
    .map((item) => `--env ${shellArg(`${item.key}=${item.value}`)}`)
    .join(' ')
  return buildDockerRun(spec, envArgs)
}

/**
 * Get error text from unknown error
 */
export function getErrorText(error: unknown, fallback: string): string {
  if (error instanceof Error) {
    return error.message
  }
  return fallback
}