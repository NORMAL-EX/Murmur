import { useCallback, useEffect, useState } from 'react'
import { Undo2, ShieldAlert } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { useAuth } from '@/contexts/AuthContext'
import { api, ApiError } from '@/lib/api'
import { toast } from '@/lib/toast'
import { formatTime } from '@/lib/format'
import type { DMAuditConversation, DMAuditThread } from '@/lib/types'

export default function DMAuditAdmin() {
  const { user: me } = useAuth()
  const [convos, setConvos] = useState<DMAuditConversation[]>([])
  const [loading, setLoading] = useState(false)
  const [active, setActive] = useState<{ a: number; b: number } | null>(null)
  const [thread, setThread] = useState<DMAuditThread | null>(null)
  const [loadingThread, setLoadingThread] = useState(false)

  const loadConvos = useCallback(async () => {
    setLoading(true)
    try {
      setConvos(await api.admin.dmConversations())
    } catch (e) {
      toast.error('加载失败', e instanceof ApiError ? e.message : undefined)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (me?.role === 'super_admin') void loadConvos()
  }, [me?.role, loadConvos])

  const openThread = useCallback(async (a: number, b: number) => {
    setActive({ a, b })
    setLoadingThread(true)
    try {
      setThread(await api.admin.dmThread(a, b))
    } catch (e) {
      toast.error('加载失败', e instanceof ApiError ? e.message : undefined)
    } finally {
      setLoadingThread(false)
    }
  }, [])

  const onRecall = async (id: number) => {
    try {
      await api.recallDm(id)
      toast.success('已撤回')
      if (active) void openThread(active.a, active.b)
    } catch (e) {
      toast.error('撤回失败', e instanceof ApiError ? e.message : undefined)
    }
  }

  if (me?.role !== 'super_admin') {
    return (
      <div className="flex flex-col items-center gap-2 py-16 text-muted-foreground">
        <ShieldAlert className="size-8" />
        仅系统管理员可访问私信审查
      </div>
    )
  }

  const nameOf = (id: number) => {
    if (!thread) return ''
    const u = thread.user_a.id === id ? thread.user_a : thread.user_b
    return u.nickname || u.username
  }

  return (
    <div className="flex flex-col gap-4">
      <div>
        <h1 className="font-bold text-2xl">私信审查</h1>
        <p className="text-muted-foreground text-sm">
          查看任意成员之间的私信往来,可撤回违规消息(含已被撤回的原文)。仅系统管理员可见。
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-[300px_1fr]">
        <div className="flex max-h-[72vh] flex-col gap-1 overflow-y-auto rounded-lg border p-2">
          {loading && <div className="p-3 text-muted-foreground text-sm">加载中…</div>}
          {!loading && convos.length === 0 && (
            <div className="p-3 text-muted-foreground text-sm">暂无私信</div>
          )}
          {convos.map((c) => {
            const on = active?.a === c.user_a.id && active?.b === c.user_b.id
            return (
              <button
                key={`${c.user_a.id}-${c.user_b.id}`}
                type="button"
                onClick={() => openThread(c.user_a.id, c.user_b.id)}
                className={`flex flex-col gap-0.5 rounded-md p-2 text-left hover:bg-accent ${on ? 'bg-accent' : ''}`}
              >
                <div className="flex items-center gap-1 font-medium text-sm">
                  <span className="truncate">{c.user_a.nickname || c.user_a.username}</span>
                  <span className="text-muted-foreground">↔</span>
                  <span className="truncate">{c.user_b.nickname || c.user_b.username}</span>
                  <span className="ml-auto shrink-0 text-muted-foreground text-xs">{c.count}</span>
                </div>
                <div className="truncate text-muted-foreground text-xs">{c.preview}</div>
              </button>
            )
          })}
        </div>

        <div className="flex max-h-[72vh] min-h-[40vh] flex-col overflow-y-auto rounded-lg border p-3">
          {!active && (
            <div className="m-auto text-muted-foreground text-sm">选择左侧一组对话查看内容</div>
          )}
          {active && loadingThread && (
            <div className="m-auto text-muted-foreground text-sm">加载中…</div>
          )}
          {active && !loadingThread && thread && (
            <div className="flex flex-col gap-2">
              {thread.items.map((m) => (
                <div key={m.id} className="group rounded-md border border-border p-2">
                  <div className="mb-0.5 flex items-center gap-2 text-xs">
                    <span className="font-medium">{nameOf(m.sender_id)}</span>
                    <span className="text-muted-foreground">{formatTime(m.created_at)}</span>
                    {m.recalled && (
                      <Badge variant="destructive" className="h-4 px-1 text-[10px]">
                        已撤回
                      </Badge>
                    )}
                    {!m.recalled && (
                      <button
                        type="button"
                        onClick={() => onRecall(m.id)}
                        className="ml-auto flex items-center gap-1 text-muted-foreground opacity-0 transition-opacity hover:text-destructive group-hover:opacity-100"
                      >
                        <Undo2 className="size-3.5" />
                        撤回
                      </button>
                    )}
                  </div>
                  <div className="whitespace-pre-wrap break-words text-sm">{m.content}</div>
                </div>
              ))}
              {thread.items.length === 0 && (
                <div className="text-muted-foreground text-sm">没有消息</div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
