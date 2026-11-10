import { useMemo } from 'react'
import type { PricingRate } from '../../../api/admin'

export interface UsePricingOptions {
  rates: PricingRate[]
  effectiveNodeId: string
  cpuCores: number
  memoryGB: number
  diskGB: number
  durationHours: number
  buildMode: 'dockerfile' | 'compose'
  selectedServices: string[]
}

export interface UsePricingReturn {
  priceEstimate: number | null
}

/**
 * Hook for calculating price estimate
 */
export function usePricing(options: UsePricingOptions): UsePricingReturn {
  const { rates, effectiveNodeId, cpuCores, memoryGB, diskGB, durationHours, buildMode, selectedServices } = options

  const activeServices = buildMode === 'compose' ? selectedServices : []
  const serviceCount = buildMode === 'compose' ? activeServices.length : 1

  const priceEstimate = useMemo(() => {
    const rate = rates.find((r) => r.agent_id === effectiveNodeId)
    if (!rate) {
      return null
    }
    const hourly = cpuCores * rate.cpu_per_core_hour + memoryGB * rate.mem_per_gb_hour + diskGB * rate.disk_per_gb_hour
    return Math.round(hourly * durationHours * serviceCount * 100) / 100
  }, [rates, effectiveNodeId, cpuCores, memoryGB, diskGB, durationHours, serviceCount])

  return { priceEstimate }
}