// Node selection utilities

import type { WorkerNode } from '../types/workerNode'
import { percent } from '../types/workerNode'

/**
 * Get worker display name by ID
 */
export function getWorkerName(nodes: { id: string; name: string }[], id: string): string {
  const foundNode = nodes.find((node) => node.id === id)
  if (foundNode) {
    return foundNode.name
  }
  return id
}

/**
 * Calculate combined load score for a node (CPU + memory)
 */
export function getNodeLoad(node: WorkerNode): number {
  const cpuLoad = percent(node.cpu.used, node.cpu.total)
  const memoryLoad = percent(node.memory.used, node.memory.total)
  return cpuLoad + memoryLoad
}

/**
 * Find least loaded node from online nodes
 */
export function findLeastLoadedNode(onlineNodes: WorkerNode[]): WorkerNode | undefined {
  const sortedByLoad = [...onlineNodes].sort((first, second) => getNodeLoad(first) - getNodeLoad(second))
  return sortedByLoad[0]
}