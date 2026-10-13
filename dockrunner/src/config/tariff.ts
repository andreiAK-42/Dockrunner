export interface TariffPlan {
  id: string
  name: string
  price: string
  apps: number
  domains: number
  ports: number
  envConfigs: number
  dbStorageGB: number
}

export const TARIFF_PLANS: TariffPlan[] = [
  { id: 'student', name: 'Студенческий', price: 'Бесплатно', apps: 10, domains: 5, ports: 5, envConfigs: 10, dbStorageGB: 10 },
  { id: 'basic', name: 'Базовый', price: '990 ₽/мес', apps: 30, domains: 15, ports: 10, envConfigs: 30, dbStorageGB: 50 },
  { id: 'pro', name: 'Про', price: '2990 ₽/мес', apps: 100, domains: 50, ports: 25, envConfigs: 100, dbStorageGB: 200 },
]

export const TARIFF: TariffPlan = TARIFF_PLANS[0]

export function getTariff(id: string): TariffPlan {
  return TARIFF_PLANS.find((t) => t.id === id) || TARIFF_PLANS[0]
}
