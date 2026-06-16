import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Hash, Lock, Search, PanelLeft, Circle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import {
  Dialog,
  DialogTrigger,
  DialogPopup,
  DialogHeader,
  DialogTitle,
  DialogPanel,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { useChat } from '@/contexts/ChatContext'
import { api } from '@/lib/api'
import { initials, formatRelative } from '@/lib/format'
import type { Message } from '@/lib/types'

function SearchDialog({ channelId }: { channelId: number }) {
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const [results, setResults] = useState<Message[]>([])
  const [loading, setLoading] = useState(false)

  const run = async (value: string) => {
    setQ(value)
    if (!value.trim()) {
      setResults([])
      return
    }
    setLoading(true)
    try {
      setResults(await api.searchMessages(channelId, value.trim()))
    } catch {
      setResults([])
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <Button variant="outline" size="icon" aria-label="搜索消息">
            <Search />
          </Button>
        }
      />
      <DialogPopup className="max-w-lg">
        <DialogHeader>
          <DialogTitle>搜索消息</DialogTitle>
        </DialogHeader>
        <DialogPanel>
          <Input
            autoFocus
            value={q}
            onChange={(e) => run(e.target.value)}
            placeholder="输入关键词搜索本频道消息"
          />
          <div className="mt-3 flex max-h-80 flex-col gap-2 overflow-y-auto">
            {loading && <div className="text-muted-foreground text-sm">搜索中…</div>}
            {!loading && q && results.length === 0 && (
              <div className="text-muted-foreground text-sm">没有匹配的消息</div>
            )}
            {results.map((m) => (
              <div key={m.id} className="rounded-lg border p-2">
                <div className="flex items-center justify-between">
                  <span className="font-medium text-sm">
                    {m.sender?.nickname || m.sender?.username || '未知'}
                  </span>
                  <span className="text-muted-foreground text-xs">{formatRelative(m.created_at)}</span>
                </div>
                <p className="mt-0.5 break-words text-sm">{m.content}</p>
              </div>
            ))}
          </div>
        </DialogPanel>
      </DialogPopup>
    </Dialog>
  )
}

export function ConversationHeader({ onOpenSidebar }: { onOpenSidebar?: () => void }) {
  const { view, channels, membersById, conversations, onlineIds } = useChat()
  const navigate = useNavigate()

  let title: React.ReactNode = null
  let subtitle: React.ReactNode = null
  let channelId: number | null = null

  if (view?.type === 'channel') {
    const ch = channels.find((c) => c.id === view.id)
    channelId = ch?.id ?? null
    title = (
      <span className="flex items-center gap-1.5">
        {ch?.readonly ? <Lock className="size-4" /> : <Hash className="size-4" />}
        {ch?.name || '频道'}
        {ch?.readonly && (
          <Badge variant="outline" className="ml-1 text-[10px]">
            只读
          </Badge>
        )}
      </span>
    )
    subtitle = ch?.description || `在线 ${onlineIds.size} 人`
  } else if (view?.type === 'dm') {
    const u = membersById.get(view.userId) || conversations.find((c) => c.user.id === view.userId)?.user
    const online = onlineIds.has(view.userId)
    title = (
      <button
        type="button"
        onClick={() => navigate(`/u/${view.userId}`)}
        className="flex items-center gap-2 hover:underline"
      >
        <Avatar className="size-6">
          <AvatarImage src={u?.avatar_url || undefined} />
          <AvatarFallback>{initials(u?.nickname || u?.username || '?')}</AvatarFallback>
        </Avatar>
        {u?.nickname || u?.username || '用户'}
      </button>
    )
    subtitle = (
      <span className="flex items-center gap-1">
        <Circle className={`size-2 ${online ? 'fill-success text-success' : 'fill-muted-foreground text-muted-foreground'}`} />
        {online ? '在线' : '离线'}
      </span>
    )
  }

  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-border border-b px-4">
      <Button
        variant="outline"
        size="icon"
        className="md:hidden"
        onClick={onOpenSidebar}
        aria-label="打开侧栏"
      >
        <PanelLeft />
      </Button>
      <div className="min-w-0 flex-1">
        <div className="truncate font-semibold">{title}</div>
        <div className="truncate text-muted-foreground text-xs">{subtitle}</div>
      </div>
      {channelId !== null && <SearchDialog channelId={channelId} />}
    </header>
  )
}

export default ConversationHeader
