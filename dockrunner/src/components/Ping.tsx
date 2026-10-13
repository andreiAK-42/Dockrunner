import { useEffect, useState } from 'react'
import { Typography } from '@mui/material'
import { measurePing } from '../utils/ping'

export function PingText({ url }: { url: string }) {
  const [measuredUrl, setMeasuredUrl] = useState<string | null>(null)
  const [pingMs, setPingMs] = useState<number | null | undefined>(undefined)

  useEffect(() => {
    if (!url) {
      return
    }

    let alive = true
    measurePing(url).then((measuredValue) => {
      if (!alive) {
        return
      }
      setMeasuredUrl(url)
      setPingMs(measuredValue)
    })

    return () => {
      alive = false
    }
  }, [url])

  if (!url) {
    return <Typography component="span" variant="caption" color="text.secondary">недоступен</Typography>
  }

  const isStale = measuredUrl !== url
  if (isStale || pingMs === undefined) {
    return <Typography component="span" variant="caption" color="text.secondary">замер...</Typography>
  }

  if (pingMs === null) {
    return <Typography component="span" variant="caption" color="text.secondary">недоступен</Typography>
  }

  return <Typography component="span" variant="caption" sx={{ fontWeight: 700 }}>{pingMs} мс</Typography>
}
