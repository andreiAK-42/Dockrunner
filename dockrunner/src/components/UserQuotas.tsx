import { Box, Typography, LinearProgress, Stack, Skeleton } from '@mui/material'
import { fetchQuotas } from '../api/quotas'
import type { Quotas } from '../api/quotas'
import { useEffect, useState } from 'react'

interface QuotaRow {
  label: string
  used: number
  total: number
  color: 'primary' | 'warning' | 'success' | 'info'
}

function toRows(quotas: Quotas): QuotaRow[] {
  const storageUsed = Math.round(quotas.storage_gb_used * 10) / 10

  return [
    { label: 'Приложений', used: quotas.apps_used, total: quotas.apps_limit, color: 'primary' },
    { label: 'Портов', used: quotas.ports_used, total: quotas.ports_limit, color: 'warning' },
    { label: 'Env-конфигов', used: quotas.env_used, total: quotas.env_limit, color: 'success' },
    { label: 'Место (ГБ)', used: storageUsed, total: quotas.storage_gb_limit, color: 'info' },
  ]
}

function formatValue(used: number, total: number) {
  if (Number.isInteger(used)) {
    return `${used} / ${total}`
  }
  return `${used.toFixed(1)} / ${total}`
}

function quotaPercentage(used: number, total: number) {
  if (total <= 0) {
    return 0
  }
  const rawPercentage = (used / total) * 100
  return Math.min(100, rawPercentage)
}

const SKELETON_ROWS = [0, 1, 2, 3]

export default function UserQuotas() {
  const [quotas, setQuotas] = useState<QuotaRow[] | null>(null)

  useEffect(() => {
    fetchQuotas()
      .then((res) => setQuotas(toRows(res.data)))
      .catch(() => setQuotas([]))
  }, [])

  if (!quotas) {
    return (
      <Stack spacing={3}>
        {SKELETON_ROWS.map((rowIndex) => (
          <Skeleton key={rowIndex} variant="rounded" height={32} />
        ))}
      </Stack>
    )
  }

  return (
    <Stack spacing={3}>
      {quotas.map((quota) => {
        const percentage = quotaPercentage(quota.used, quota.total)
        return (
          <Box key={quota.label}>
            <Stack direction="row" sx={{ justifyContent: 'space-between', mb: 1 }}>
              <Typography variant="body2" sx={{ fontWeight: 500 }}>{quota.label}</Typography>
              <Typography variant="body2" color="text.secondary">
                {formatValue(quota.used, quota.total)}
              </Typography>
            </Stack>
            <LinearProgress variant="determinate" value={percentage} color={quota.color} sx={{ height: 8, borderRadius: 4 }} />
          </Box>
        )
      })}
    </Stack>
  )
}
