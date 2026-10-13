const MS_PER_MINUTE = 60000
const MINUTES_PER_HOUR = 60
const HOURS_PER_DAY = 24

export function timeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()

  if (Number.isNaN(diff) || diff < 0) {
    return 'только что'
  }

  const minutes = Math.floor(diff / MS_PER_MINUTE)
  if (minutes < 1) {
    return 'только что'
  }
  if (minutes < MINUTES_PER_HOUR) {
    return `${minutes} мин назад`
  }

  const hours = Math.floor(minutes / MINUTES_PER_HOUR)
  if (hours < HOURS_PER_DAY) {
    return `${hours} ч назад`
  }

  const days = Math.floor(hours / HOURS_PER_DAY)
  return `${days} дн назад`
}
