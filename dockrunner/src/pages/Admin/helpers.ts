import type { TariffPlan } from '../../api/admin'

export function formatExpiry(iso: string | null) {
  if (!iso) return 'Бессрочно'
  return new Date(iso).toLocaleDateString('ru-RU')
}

export function planName(plans: TariffPlan[], planId: string) {
  const foundPlan = plans.find((plan) => plan.id === planId)
  if (foundPlan) {
    return foundPlan.name
  }
  return planId
}

export function todayPlusYear() {
  const d = new Date()
  d.setFullYear(d.getFullYear() + 1)
  return d.toISOString().slice(0, 10)
}
