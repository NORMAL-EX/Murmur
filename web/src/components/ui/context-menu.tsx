import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'

export interface CtxItem {
  label: string
  icon?: ReactNode
  onSelect: () => void
  destructive?: boolean
  separatorBefore?: boolean
}

type OpenFn = (e: React.MouseEvent, items: CtxItem[]) => void

const CtxMenuContext = createContext<OpenFn>(() => {})

/** Returns open(event, items) to show a single shared cursor-positioned menu. */
export function useCtxMenu() {
  return useContext(CtxMenuContext)
}

/** A singleton context menu — only one is ever shown, so menus never stack. */
export function ContextMenuProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<{ x: number; y: number; items: CtxItem[] } | null>(null)
  const close = useCallback(() => setState(null), [])

  const open = useCallback<OpenFn>((e, items) => {
    if (!items.length) return
    e.preventDefault()
    e.stopPropagation() // keep other open menus' document listeners from firing
    const x = Math.min(e.clientX, window.innerWidth - 210)
    const y = Math.min(e.clientY, window.innerHeight - 16 - items.length * 38)
    setState({ x: Math.max(8, x), y: Math.max(8, y), items })
  }, [])

  useEffect(() => {
    if (!state) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close()
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
  }, [state, close])

  return (
    <CtxMenuContext.Provider value={open}>
      {children}
      {state && (
        <div
          style={{ top: state.y, left: state.x }}
          onClick={(e) => e.stopPropagation()}
          className="fixed z-[100] min-w-[180px] overflow-hidden rounded-lg border border-border bg-popover p-1 text-popover-foreground shadow-lg"
        >
          {state.items.map((it, i) => (
            <div key={i}>
              {it.separatorBefore && <div className="my-1 h-px bg-border" />}
              <button
                type="button"
                onClick={() => {
                  close()
                  it.onSelect()
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
      )}
    </CtxMenuContext.Provider>
  )
}

export default ContextMenuProvider
