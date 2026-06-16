/** 取昵称首字符作为头像兜底文本。 */
export function initials(name: string): string {
  if (!name) return '?'
  const trimmed = name.trim()
  // 中文/单字:取第一个字符;英文:取首字母(最多两位)
  if (/[一-龥]/.test(trimmed[0])) return trimmed[0]
  const parts = trimmed.split(/\s+/)
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase()
  return trimmed.slice(0, 2).toUpperCase()
}

const pad = (n: number) => n.toString().padStart(2, '0')

/** HH:mm */
export function formatTime(iso: string): string {
  const d = new Date(iso)
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 完整日期时间 */
export function formatDateTime(iso: string): string {
  const d = new Date(iso)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 相对时间(几秒前 / 分钟前 / 小时前 / 日期) */
export function formatRelative(iso: string): string {
  const d = new Date(iso).getTime()
  const diff = Date.now() - d
  const sec = Math.floor(diff / 1000)
  if (sec < 60) return '刚刚'
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min} 分钟前`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr} 小时前`
  const day = Math.floor(hr / 24)
  if (day < 7) return `${day} 天前`
  return formatDateTime(iso).slice(0, 10)
}

/** 聊天列表里的日期分隔标签 */
export function dayLabel(iso: string): string {
  const d = new Date(iso)
  const today = new Date()
  const isSameDay = (a: Date, b: Date) =>
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  if (isSameDay(d, today)) return '今天'
  const yest = new Date(today)
  yest.setDate(today.getDate() - 1)
  if (isSameDay(d, yest)) return '昨天'
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}
