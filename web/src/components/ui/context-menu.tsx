import { useCallback, useEffect, useState, type ReactNode } from 'react'

export interface CtxItem {
  label: string
  icon?: ReactNode
  onSelect: () => void
  destructive?: boolean
  separatorBefore?: boolean
}

/** Tracks right-click position; clamps to the viewport so the menu stays visible. */
export function useContextMenu() {
  const [pos, setPos] = useState<{ x: number; y: number } | null>(null)
  const open = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    const x = Math.min(e.clientX, window.innerWidth - 200)
    const y = Math.min(e.clientY, window.innerHeight - 280)
    setPos({ x: Math.max(8, x), y: Math.max(8, y) })
  }, [])
  const close = useCallback(() => setPos(null), [])
  return { pos, open, close }
}

/** A lightweight cursor-positioned menu. Closes on outside click / Esc / scroll. */
export function ContextMenu({
  x,
  y,
  items,
  onClose,
}: {
  x: number
  y: number
  items: CtxItem[]
  onClose: () => void
}) {
  useEffect(() => {
    const close = () => onClose()
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    const t = window.setTimeout(() => {
      document.addEventListener('click', close)
      document.addEventListener('contextmenu', close)
      document.addEventListener('scroll', close, true)
      document.addEventListener('keydown', onKey)
    }, 0)
    return () => {
      window.clearTimeout(t)
      document.removeEventListener('click', close)
      document.removeEventListener('contextmenu', close)
      document.removeEventListener('scroll', close, true)
      document.removeEventListener('keydown', onKey)
    }
  }, [onClose])

  return (
    <div
      style={{ top: y, left: x }}
      onClick={(e) => e.stopPropagation()}
      className="fixed z-[90] min-w-[160px] overflow-hidden rounded-lg border border-border bg-popover p-1 text-popover-foreground shadow-lg"
    >
      {items.map((it, i) => (
        <div key={i}>
          {it.separatorBefore && <div className="my-1 h-px bg-border" />}
          <button
            type="button"
            onClick={() => {
              it.onSelect()
              onClose()
            }}
            className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm ${
              it.destructive
                ? 'text-destructive hover:bg-destructive/10'
                : 'hover:bg-accent hover:text-accent-foreground'
            }`}
          >
            {it.icon}
            {it.label}
          </button>
        </div>
      ))}
    </div>
  )
}

export default ContextMenu
