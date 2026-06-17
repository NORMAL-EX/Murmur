import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  MoreHorizontal,
  Pencil,
  Undo2,
  MessageSquare,
  SmilePlus,
  Reply,
  Copy,
  AtSign,
  UserRound,
  MicOff,
  Mic,
  Ban,
  ShieldCheck,
  ShieldOff,
} from 'lucide-react'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Input } from '@/components/ui/input'
import { Menu, MenuTrigger, MenuPopup, MenuItem, MenuSeparator } from '@/components/ui/menu'
import { Popover, PopoverTrigger, PopoverPopup } from '@/components/ui/popover'
import {
  Dialog,
  DialogPopup,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogPanel,
  DialogFooter,
  DialogClose,
} from '@/components/ui/dialog'
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectPopup,
  SelectItem,
} from '@/components/ui/select'
import { useCtxMenu, type CtxItem } from '@/components/ui/context-menu'
import { MarkdownContent } from '@/components/chat/MarkdownContent'
import { useChat } from '@/contexts/ChatContext'
import { useAuth } from '@/contexts/AuthContext'
import { toast } from '@/lib/toast'
import { initials, formatTime } from '@/lib/format'
import { api, ApiError } from '@/lib/api'
import type { Message, User } from '@/lib/types'

const QUICK_EMOJIS = ['👍', '❤️', '😂', '🎉', '😮', '😢', '🔥', '👀']

const MUTE_OPTIONS = [
  { label: '10 分钟', minutes: 10 },
  { label: '1 小时', minutes: 60 },
  { label: '1 天', minutes: 1440 },
  { label: '7 天', minutes: 10080 },
  { label: '永久', minutes: -1 },
]

/** Identity badge shown next to a sender's name. */
function RoleBadge({ role }: { role?: string }) {
  if (role === 'super_admin')
    return (
      <Badge className="h-4 border-transparent bg-amber-500/15 px-1 text-[10px] text-amber-600 dark:text-amber-400">
        系统管理员
      </Badge>
    )
  if (role === 'admin')
    return (
      <Badge className="h-4 border-transparent bg-sky-500/15 px-1 text-[10px] text-sky-600 dark:text-sky-400">
        管理员
      </Badge>
    )
  if (role === 'bot')
    return <Badge className="h-4 border-transparent bg-primary/15 px-1 text-[10px] text-primary">AI</Badge>
  return (
    <Badge variant="outline" className="h-4 px-1 text-[10px] text-muted-foreground">
      成员
    </Badge>
  )
}

export function MessageItem({ message }: { message: Message }) {
  const { user } = useAuth()
  const { toggleReaction, recallMessage, editMessage, selectDm, setReplyTo, requestMention } =
    useChat()
  const navigate = useNavigate()
  const openMenu = useCtxMenu()
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(message.content)
  const [revealed, setRevealed] = useState<string | null>(null)
  const [muteOpen, setMuteOpen] = useState(false)
  const [nickOpen, setNickOpen] = useState(false)

  const sender = message.sender
  const isOwn = sender?.id === user?.id
  const isSuper = user?.role === 'super_admin'
  const isAdmin = isSuper || user?.role === 'admin'
  const gone = message.deleted || message.recalled
  const withinWindow = Date.now() - new Date(message.created_at).getTime() < 40_000
  const canEdit = isOwn && !message.is_bot && !gone
  const canRecall = !gone && (isAdmin || (isOwn && !message.is_bot && withinWindow))
  const targetMuted = !!sender?.muted_until && new Date(sender.muted_until).getTime() > Date.now()
  const canManage =
    !!sender &&
    !isOwn &&
    sender.role !== 'bot' &&
    sender.role !== 'super_admin' &&
    isAdmin &&
    (isSuper || sender.role === 'user')

  const onReact = async (emoji: string) => {
    try {
      await toggleReaction(message.id, emoji)
    } catch {
      toast.error('操作失败')
    }
  }
  const onRecall = async () => {
    try {
      await recallMessage(message.id)
    } catch (e) {
      toast.error('撤回失败', e instanceof ApiError ? e.message : undefined)
    }
  }
  const onReveal = async () => {
    try {
      const { content } = await api.admin.revealMessage(message.id)
      setRevealed(content)
    } catch (e) {
      toast.error('查看失败', e instanceof ApiError ? e.message : undefined)
    }
  }
  const onSaveEdit = async () => {
    const text = draft.trim()
    if (!text) return
    try {
      await editMessage(message.id, text)
      setEditing(false)
    } catch (e) {
      toast.error('编辑失败', e instanceof ApiError ? e.message : undefined)
    }
  }
  const startReply = () =>
    setReplyTo({
      id: message.id,
      author: sender?.nickname || sender?.username || '用户',
      snippet: message.content || '[图片]',
    })
  const copyText = async () => {
    try {
      await navigator.clipboard.writeText(message.content)
      toast.success('已复制')
    } catch {
      toast.error('复制失败')
    }
  }
  const act = async (fn: () => Promise<unknown>, ok: string) => {
    try {
      await fn()
      toast.success(ok)
    } catch (e) {
      toast.error('操作失败', e instanceof ApiError ? e.message : undefined)
    }
  }

  // Right-click on the message body → message actions.
  const messageItems: CtxItem[] = []
  if (!gone) {
    messageItems.push({ label: '回复', icon: <Reply className="size-4" />, onSelect: startReply })
    messageItems.push({ label: '复制', icon: <Copy className="size-4" />, onSelect: copyText })
  }
  if (canRecall) {
    messageItems.push({
      label: '撤回',
      icon: <Undo2 className="size-4" />,
      onSelect: onRecall,
      destructive: true,
      separatorBefore: !gone,
    })
  }

  // Right-click on the avatar / name → member actions.
  const memberItems: CtxItem[] = []
  if (sender) {
    if (!isOwn && sender.role !== 'bot') {
      memberItems.push({
        label: '发私信',
        icon: <MessageSquare className="size-4" />,
        onSelect: () => selectDm(sender.id),
      })
      memberItems.push({
        label: '@ TA',
        icon: <AtSign className="size-4" />,
        onSelect: () => requestMention(sender.username),
      })
    }
    memberItems.push({
      label: '查看资料',
      icon: <UserRound className="size-4" />,
      onSelect: () => navigate(`/u/${sender.id}`),
    })
    if (canManage) {
      if (targetMuted) {
        memberItems.push({
          label: '解除禁言',
          icon: <Mic className="size-4" />,
          separatorBefore: true,
          onSelect: () => act(() => api.admin.updateUser(sender.id, { mute_minutes: 0 }), '已解除禁言'),
        })
      } else {
        memberItems.push({
          label: '禁言',
          icon: <MicOff className="size-4" />,
          separatorBefore: true,
          onSelect: () => setMuteOpen(true),
        })
      }
      if (sender.status === 'banned') {
        memberItems.push({
          label: '解除封禁',
          icon: <Undo2 className="size-4" />,
          onSelect: () => act(() => api.admin.updateUser(sender.id, { status: 'active' }), '已解封'),
        })
      } else {
        memberItems.push({
          label: '封禁',
          icon: <Ban className="size-4" />,
          destructive: true,
          onSelect: () => act(() => api.admin.updateUser(sender.id, { status: 'banned' }), '已封禁'),
        })
      }
      memberItems.push({
        label: '修改群昵称',
        icon: <Pencil className="size-4" />,
        onSelect: () => setNickOpen(true),
      })
    }
    if (isSuper && !isOwn && sender.role === 'user') {
      memberItems.push({
        label: '设为管理员',
        icon: <ShieldCheck className="size-4" />,
        separatorBefore: true,
        onSelect: () => act(() => api.admin.updateUser(sender.id, { role: 'admin' }), '已设为管理员'),
      })
    }
    if (isSuper && sender.role === 'admin') {
      memberItems.push({
        label: '取消管理员',
        icon: <ShieldOff className="size-4" />,
        separatorBefore: true,
        onSelect: () => act(() => api.admin.updateUser(sender.id, { role: 'user' }), '已取消管理员'),
      })
    }
  }

  const scrollToReply = () => {
    if (!message.reply_to) return
    const el = document.getElementById(`msg-${message.reply_to.id}`)
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' })
      el.classList.add('bg-primary/10')
      setTimeout(() => el.classList.remove('bg-primary/10'), 1200)
    }
  }

  return (
    <div
      id={`msg-${message.id}`}
      className="group flex gap-3 px-4 py-1.5 transition-colors hover:bg-muted/40"
    >
      <button
        type="button"
        onClick={() => sender && navigate(`/u/${sender.id}`)}
        onContextMenu={(e) => openMenu(e, memberItems)}
        className="mt-0.5 shrink-0"
      >
        <Avatar className="size-9">
          <AvatarImage src={sender?.avatar_url || undefined} alt={sender?.nickname} />
          <AvatarFallback>{initials(sender?.nickname || sender?.username || '?')}</AvatarFallback>
        </Avatar>
      </button>

      <div className="min-w-0 flex-1" onContextMenu={(e) => openMenu(e, messageItems)}>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => sender && navigate(`/u/${sender.id}`)}
            onContextMenu={(e) => openMenu(e, memberItems)}
            className="font-medium text-sm hover:underline"
          >
            {sender?.nickname || sender?.username || '未知用户'}
          </button>
          <RoleBadge role={sender?.role} />
          <span className="text-muted-foreground text-xs">{formatTime(message.created_at)}</span>
          {message.edited && <span className="text-muted-foreground text-xs">(已编辑)</span>}
        </div>

        {message.reply_to && !gone && (
          <button
            type="button"
            onClick={scrollToReply}
            className="mb-0.5 flex max-w-full items-center gap-1 rounded border-primary border-l-2 bg-muted/50 px-2 py-0.5 text-left text-muted-foreground text-xs hover:bg-muted"
          >
            <Reply className="size-3 shrink-0" />
            <span className="shrink-0 font-medium text-foreground/80">{message.reply_to.sender_name}</span>
            <span className="truncate">
              {message.reply_to.recalled ? '[已撤回]' : message.reply_to.content}
            </span>
          </button>
        )}

        {message.recalled ? (
          <div className="text-muted-foreground text-sm italic">
            此消息已被撤回
            {isSuper &&
              (revealed === null ? (
                <button
                  type="button"
                  onClick={onReveal}
                  className="ml-2 text-primary not-italic hover:underline"
                >
                  点击查看
                </button>
              ) : (
                <div className="mt-1 rounded-md border border-dashed border-border p-2 not-italic">
                  <MarkdownContent content={revealed} />
                </div>
              ))}
          </div>
        ) : message.deleted ? (
          <p className="text-muted-foreground text-sm italic">(消息已删除)</p>
        ) : editing ? (
          <div className="mt-1 flex flex-col gap-2">
            <Textarea
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              rows={2}
              autoFocus
              onKeyDown={(e) => {
                if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) onSaveEdit()
                if (e.key === 'Escape') setEditing(false)
              }}
            />
            <div className="flex gap-2">
              <Button size="sm" variant="default" onClick={onSaveEdit}>
                保存
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => {
                  setEditing(false)
                  setDraft(message.content)
                }}
              >
                取消
              </Button>
              <span className="self-center text-muted-foreground text-xs">Ctrl+Enter 保存 · Esc 取消</span>
            </div>
          </div>
        ) : (
          <MarkdownContent content={message.content} />
        )}

        {message.reactions.length > 0 && !gone && (
          <div className="mt-1 flex flex-wrap gap-1">
            {message.reactions.map((r) => (
              <Button
                key={r.emoji}
                size="xs"
                variant={r.reacted || r.user_ids.includes(user?.id ?? -1) ? 'secondary' : 'outline'}
                className="h-6 gap-1 px-1.5"
                onClick={() => onReact(r.emoji)}
              >
                <span>{r.emoji}</span>
                <span className="text-xs">{r.count}</span>
              </Button>
            ))}
          </div>
        )}
      </div>

      {!gone && !editing && (
        <div className="flex items-start gap-1 self-start opacity-0 transition-opacity group-hover:opacity-100">
          <Button
            variant="outline"
            size="icon-xs"
            aria-label="回复"
            onClick={startReply}
          >
            <Reply />
          </Button>
          <Popover>
            <PopoverTrigger
              render={
                <Button variant="outline" size="icon-xs" aria-label="添加表情">
                  <SmilePlus />
                </Button>
              }
            />
            <PopoverPopup className="w-auto" align="end">
              <div className="flex gap-1">
                {QUICK_EMOJIS.map((emoji) => (
                  <button
                    key={emoji}
                    type="button"
                    onClick={() => onReact(emoji)}
                    className="rounded-md p-1 text-lg hover:bg-accent"
                  >
                    {emoji}
                  </button>
                ))}
              </div>
            </PopoverPopup>
          </Popover>

          <Menu>
            <MenuTrigger
              render={
                <Button variant="outline" size="icon-xs" aria-label="更多操作">
                  <MoreHorizontal />
                </Button>
              }
            />
            <MenuPopup className="min-w-[140px] menu-popup-animated" align="end">
              {canEdit && (
                <MenuItem onClick={() => setEditing(true)} className="flex items-center gap-2">
                  <Pencil className="size-4" />
                  编辑
                </MenuItem>
              )}
              {canRecall && (
                <>
                  {canEdit && <MenuSeparator />}
                  <MenuItem onClick={onRecall} variant="destructive" className="flex items-center gap-2">
                    <Undo2 className="size-4" />
                    撤回
                  </MenuItem>
                </>
              )}
            </MenuPopup>
          </Menu>
        </div>
      )}

      {muteOpen && sender && <MemberMuteDialog user={sender} onClose={() => setMuteOpen(false)} />}
      {nickOpen && sender && <MemberNickDialog user={sender} onClose={() => setNickOpen(false)} />}
    </div>
  )
}

function MemberMuteDialog({ user, onClose }: { user: User; onClose: () => void }) {
  const [minutes, setMinutes] = useState('60')
  const [saving, setSaving] = useState(false)
  const save = async () => {
    setSaving(true)
    try {
      await api.admin.updateUser(user.id, { mute_minutes: Number(minutes) })
      toast.success('已禁言')
      onClose()
    } catch (e) {
      toast.error('操作失败', e instanceof ApiError ? e.message : undefined)
    } finally {
      setSaving(false)
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogPopup className="max-w-xs">
        <DialogHeader>
          <DialogTitle>禁言 · {user.nickname || user.username}</DialogTitle>
          <DialogDescription>禁言期间该成员可浏览但不能发送消息。</DialogDescription>
        </DialogHeader>
        <DialogPanel>
          <Select value={minutes} onValueChange={(v) => setMinutes(v as string)}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectPopup>
              {MUTE_OPTIONS.map((o) => (
                <SelectItem key={o.minutes} value={String(o.minutes)}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectPopup>
          </Select>
        </DialogPanel>
        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>取消</DialogClose>
          <Button variant="default" onClick={save} disabled={saving}>
            确定禁言
          </Button>
        </DialogFooter>
      </DialogPopup>
    </Dialog>
  )
}

function MemberNickDialog({ user, onClose }: { user: User; onClose: () => void }) {
  const [nick, setNick] = useState(user.nickname || '')
  const [saving, setSaving] = useState(false)
  const save = async () => {
    if (!nick.trim()) {
      toast.error('昵称不能为空')
      return
    }
    setSaving(true)
    try {
      await api.admin.updateUser(user.id, { nickname: nick.trim() })
      toast.success('已修改群昵称')
      onClose()
    } catch (e) {
      toast.error('操作失败', e instanceof ApiError ? e.message : undefined)
    } finally {
      setSaving(false)
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogPopup className="max-w-xs">
        <DialogHeader>
          <DialogTitle>修改群昵称 · @{user.username}</DialogTitle>
        </DialogHeader>
        <DialogPanel>
          <Input value={nick} onChange={(e) => setNick(e.target.value)} placeholder="新的群昵称" autoFocus />
        </DialogPanel>
        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>取消</DialogClose>
          <Button variant="default" onClick={save} disabled={saving}>
            保存
          </Button>
        </DialogFooter>
      </DialogPopup>
    </Dialog>
  )
}

export default MessageItem
