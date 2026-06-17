import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Loader2, Undo2, Reply } from 'lucide-react'
import { MessageItem } from '@/components/chat/MessageItem'
import { MarkdownContent } from '@/components/chat/MarkdownContent'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { useChat } from '@/contexts/ChatContext'
import { useAuth } from '@/contexts/AuthContext'
import { api, ApiError } from '@/lib/api'
import { toast } from '@/lib/toast'
import { initials, formatTime, dayLabel } from '@/lib/format'
import type { DirectMessage } from '@/lib/types'

function DayDivider({ label }: { label: string }) {
  return (
    <div className="my-3 flex items-center gap-3 px-4">
      <div className="h-px flex-1 bg-border" />
      <span className="text-muted-foreground text-xs">{label}</span>
      <div className="h-px flex-1 bg-border" />
    </div>
  )
}

function DMBubble({ dm }: { dm: DirectMessage }) {
  const { user } = useAuth()
  const { recallDm, setReplyTo } = useChat()
  const navigate = useNavigate()
  const [revealed, setRevealed] = useState<string | null>(null)
  const mine = dm.sender_id === user?.id
  const sender = dm.sender
  const isSuper = user?.role === 'super_admin'
  const isAdmin = isSuper || user?.role === 'admin'
  const withinWindow = Date.now() - new Date(dm.created_at).getTime() < 40_000
  const canRecall = !dm.recalled && (isAdmin || (mine && withinWindow))

  const onRecall = async () => {
    try {
      await recallDm(dm.id)
    } catch (e) {
      toast.error('撤回失败', e instanceof ApiError ? e.message : undefined)
    }
  }
  const onReveal = async () => {
    try {
      const { content } = await api.admin.revealDm(dm.id)
      setRevealed(content)
    } catch (e) {
      toast.error('查看失败', e instanceof ApiError ? e.message : undefined)
    }
  }
  const startReply = () =>
    setReplyTo({
      id: dm.id,
      author: mine ? '我' : sender?.nickname || sender?.username || '对方',
      snippet: dm.content || '[图片]',
    })

  const replyEl =
    dm.reply_to && !dm.recalled ? (
      <div className="mb-1 truncate border-current/40 border-l-2 pl-1.5 text-xs opacity-80">
        <span className="font-medium">{dm.reply_to.sender_name}</span>:{' '}
        {dm.reply_to.recalled ? '[已撤回]' : dm.reply_to.content}
      </div>
    ) : null

  const body = dm.recalled ? (
    <span className="text-sm italic opacity-80">
      此消息已被撤回
      {isSuper &&
        (revealed === null ? (
          <button
            type="button"
            onClick={onReveal}
            className="ml-2 underline not-italic opacity-100 hover:opacity-80"
          >
            点击查看
          </button>
        ) : (
          <span className="mt-1 block whitespace-pre-wrap not-italic">{revealed}</span>
        ))}
    </span>
  ) : (
    <MarkdownContent content={dm.content} />
  )

  const actions = (
    <div className="flex items-center gap-0.5 self-end opacity-0 transition-opacity group-hover:opacity-100">
      {!dm.recalled && (
        <button
          type="button"
          onClick={startReply}
          aria-label="回复"
          className="p-1 text-muted-foreground hover:text-foreground"
        >
          <Reply className="size-3.5" />
        </button>
      )}
      {canRecall && (
        <button
          type="button"
          onClick={onRecall}
          aria-label="撤回"
          className="p-1 text-muted-foreground hover:text-foreground"
        >
          <Undo2 className="size-3.5" />
        </button>
      )}
    </div>
  )

  if (mine) {
    return (
      <div className="group flex items-end justify-end gap-1 px-4 py-1">
        {actions}
        <div className="max-w-[75%] rounded-2xl rounded-br-sm bg-primary px-3 py-2 text-primary-foreground">
          {replyEl}
          {body}
          <div className="mt-0.5 text-right text-[10px] text-primary-foreground/70">
            {formatTime(dm.created_at)}
          </div>
        </div>
      </div>
    )
  }
  return (
    <div className="group flex items-end gap-2 px-4 py-1">
      <button type="button" onClick={() => sender && navigate(`/u/${sender.id}`)} className="shrink-0 self-end">
        <Avatar className="size-7">
          <AvatarImage src={sender?.avatar_url || undefined} />
          <AvatarFallback>{initials(sender?.nickname || sender?.username || '?')}</AvatarFallback>
        </Avatar>
      </button>
      <div className="max-w-[75%] rounded-2xl rounded-bl-sm bg-muted px-3 py-2">
        {replyEl}
        {body}
        <div className="mt-0.5 text-[10px] text-muted-foreground">{formatTime(dm.created_at)}</div>
      </div>
      {actions}
    </div>
  )
}

export function MessageList() {
  const {
    view,
    messages,
    dmMessages,
    channelHasMore,
    dmHasMore,
    loadingMessages,
    loadMore,
    typingUserIds,
    membersById,
  } = useChat()

  const isChannel = view?.type === 'channel'
  const items = isChannel ? messages : dmMessages
  const hasMore = isChannel ? channelHasMore : dmHasMore

  const scrollRef = useRef<HTMLDivElement>(null)
  const stick = useRef(true)
  const loadingMore = useRef(false)
  const prevHeight = useRef(0)

  // Reset sticky-to-bottom whenever the conversation changes.
  useEffect(() => {
    stick.current = true
  }, [view?.type, isChannel ? view?.id : view?.userId])

  useLayoutEffect(() => {
    const el = scrollRef.current
    if (!el) return
    if (loadingMore.current) {
      el.scrollTop = el.scrollHeight - prevHeight.current
      loadingMore.current = false
    } else if (stick.current) {
      el.scrollTop = el.scrollHeight
    }
  }, [items])

  const onScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const el = e.currentTarget
    stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80
    if (el.scrollTop < 60 && hasMore && !loadingMore.current) {
      prevHeight.current = el.scrollHeight
      loadingMore.current = true
      loadMore()
    }
  }

  const typingNames = typingUserIds
    .map((id) => membersById.get(id)?.nickname || membersById.get(id)?.username)
    .filter(Boolean)

  let lastDay = ''

  return (
    <div ref={scrollRef} onScroll={onScroll} className="flex-1 overflow-y-auto">
      {hasMore && (
        <div className="flex justify-center py-2 text-muted-foreground text-xs">
          <Loader2 className="size-4 animate-spin" />
        </div>
      )}
      {!hasMore && items.length > 0 && (
        <div className="py-3 text-center text-muted-foreground text-xs">没有更多消息了</div>
      )}
      {loadingMessages && items.length === 0 && (
        <div className="flex h-full items-center justify-center text-muted-foreground text-sm">
          加载中…
        </div>
      )}
      {!loadingMessages && items.length === 0 && (
        <div className="flex h-full items-center justify-center text-muted-foreground text-sm">
          还没有消息,来说点什么吧
        </div>
      )}

      <div className="flex flex-col py-2">
        {isChannel
          ? messages.map((m) => {
              const d = dayLabel(m.created_at)
              const showDay = d !== lastDay
              lastDay = d
              return (
                <div key={m.id}>
                  {showDay && <DayDivider label={d} />}
                  <MessageItem message={m} />
                </div>
              )
            })
          : dmMessages.map((m) => {
              const d = dayLabel(m.created_at)
              const showDay = d !== lastDay
              lastDay = d
              return (
                <div key={m.id}>
                  {showDay && <DayDivider label={d} />}
                  <DMBubble dm={m} />
                </div>
              )
            })}
      </div>

      {typingNames.length > 0 && (
        <div className="px-4 pb-2 text-muted-foreground text-xs">
          {typingNames.join('、')} 正在输入…
        </div>
      )}
    </div>
  )
}

export default MessageList
