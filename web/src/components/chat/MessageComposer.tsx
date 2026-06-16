import { useEffect, useMemo, useRef, useState } from 'react'
import { ImagePlus, Loader2, Send, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { api, ApiError } from '@/lib/api'
import { toast } from '@/lib/toast'
import { useChat } from '@/contexts/ChatContext'
import { useAuth } from '@/contexts/AuthContext'
import { useSettings } from '@/contexts/SettingsContext'
import { initials } from '@/lib/format'
import type { User } from '@/lib/types'

function detectMention(value: string, caret: number) {
  const upto = value.slice(0, caret)
  const m = upto.match(/(^|\s)@(\w{0,32})$/)
  if (!m) return null
  const query = m[2]
  return { query, start: caret - query.length - 1 }
}

export function MessageComposer() {
  const { view, channels, members, sendMessage, sendDm, sendTyping, cooldownUntil, connected } =
    useChat()
  const { user } = useAuth()
  const { settings } = useSettings()
  const taRef = useRef<HTMLTextAreaElement>(null)

  const [text, setText] = useState('')
  const [mention, setMention] = useState<{ query: string; start: number } | null>(null)
  const [mentionIdx, setMentionIdx] = useState(0)
  const [now, setNow] = useState(Date.now())
  const [image, setImage] = useState<File | null>(null)
  const [uploading, setUploading] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  const previewUrl = useMemo(() => (image ? URL.createObjectURL(image) : null), [image])
  useEffect(() => {
    return () => {
      if (previewUrl) URL.revokeObjectURL(previewUrl)
    }
  }, [previewUrl])

  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin'
  const isChannel = view?.type === 'channel'
  const channel = isChannel ? channels.find((c) => c.id === view.id) : undefined
  const readonlyBlocked = !!channel?.readonly && !isAdmin

  const cooldownLeft = Math.max(0, Math.ceil((cooldownUntil - now) / 1000))
  useEffect(() => {
    if (cooldownUntil <= Date.now()) return
    const t = setInterval(() => setNow(Date.now()), 500)
    return () => clearInterval(t)
  }, [cooldownUntil])

  const candidates = useMemo(() => {
    if (!mention) return []
    const q = mention.query.toLowerCase()
    return members
      .filter((m) => m.id !== user?.id)
      .filter(
        (m) =>
          m.username.toLowerCase().includes(q) || (m.nickname || '').toLowerCase().includes(q),
      )
      .slice(0, 6)
  }, [mention, members, user?.id])

  const maxLen = settings.max_message_length || 2000
  const tooLong = text.length > maxLen

  const insertMention = (u: User) => {
    if (!mention) return
    const before = text.slice(0, mention.start)
    const after = text.slice(mention.start + 1 + mention.query.length)
    const inserted = `@${u.username} `
    const next = before + inserted + after
    setText(next)
    setMention(null)
    requestAnimationFrame(() => {
      const pos = before.length + inserted.length
      taRef.current?.focus()
      taRef.current?.setSelectionRange(pos, pos)
    })
  }

  const pickImage = (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0]
    e.target.value = ''
    if (!f) return
    if (!f.type.startsWith('image/')) {
      toast.error('仅支持图片文件')
      return
    }
    if (f.size > 8 << 20) {
      toast.error('图片不能超过 8MB')
      return
    }
    setImage(f)
  }

  const submit = async () => {
    if (uploading || cooldownLeft > 0 || readonlyBlocked || !connected) return
    const body = text.trim()
    if ((!body && !image) || tooLong) return

    let content = body
    if (image) {
      setUploading(true)
      try {
        const { url } = await api.uploadImage(image)
        content = body ? `${body}\n![图片](${url})` : `![图片](${url})`
      } catch (err) {
        toast.error('图片上传失败', err instanceof ApiError ? err.message : undefined)
        setUploading(false)
        return
      }
      setUploading(false)
    }

    if (isChannel) sendMessage(content)
    else if (view?.type === 'dm') sendDm(content)
    setText('')
    setImage(null)
    setMention(null)
  }

  const onChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const v = e.target.value
    setText(v)
    sendTyping()
    const caret = e.target.selectionStart ?? v.length
    setMention(detectMention(v, caret))
    setMentionIdx(0)
  }

  const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (mention && candidates.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setMentionIdx((i) => (i + 1) % candidates.length)
        return
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        setMentionIdx((i) => (i - 1 + candidates.length) % candidates.length)
        return
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault()
        insertMention(candidates[mentionIdx])
        return
      }
      if (e.key === 'Escape') {
        setMention(null)
        return
      }
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      submit()
    }
  }

  if (readonlyBlocked) {
    return (
      <div className="border-border border-t px-4 py-4 text-center text-muted-foreground text-sm">
        该频道为只读,仅管理员可发言
      </div>
    )
  }

  const disabled = !connected || cooldownLeft > 0

  return (
    <div className="relative border-border border-t p-3">
      {mention && candidates.length > 0 && (
        <div className="absolute bottom-full left-3 mb-1 w-64 overflow-hidden rounded-lg border bg-popover shadow-lg">
          {candidates.map((u, i) => (
            <button
              key={u.id}
              type="button"
              onMouseDown={(e) => {
                e.preventDefault()
                insertMention(u)
              }}
              className={`flex w-full items-center gap-2 px-2 py-1.5 text-left text-sm ${
                i === mentionIdx ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/50'
              }`}
            >
              <Avatar className="size-6">
                <AvatarImage src={u.avatar_url || undefined} />
                <AvatarFallback>{initials(u.nickname || u.username)}</AvatarFallback>
              </Avatar>
              <span className="font-medium">{u.nickname || u.username}</span>
              <span className="text-muted-foreground text-xs">@{u.username}</span>
              {u.role === 'bot' && <span className="ml-auto text-[10px] text-primary">BOT</span>}
            </button>
          ))}
        </div>
      )}

      {image && previewUrl && (
        <div className="mb-2 flex items-center gap-2">
          <div className="relative">
            <img
              src={previewUrl}
              alt="预览"
              className="size-16 rounded-lg border border-border object-cover"
            />
            <button
              type="button"
              onClick={() => setImage(null)}
              className="-right-1.5 -top-1.5 absolute flex size-5 items-center justify-center rounded-full bg-destructive text-white"
              aria-label="移除图片"
            >
              <X className="size-3" />
            </button>
          </div>
          <span className="truncate text-muted-foreground text-xs">{image.name}</span>
        </div>
      )}

      <div className="flex items-end gap-2">
        <input ref={fileRef} type="file" accept="image/*" hidden onChange={pickImage} />
        <Button
          variant="ghost"
          size="icon"
          onClick={() => fileRef.current?.click()}
          disabled={disabled || uploading}
          aria-label="发送图片"
          className="text-muted-foreground hover:text-foreground"
        >
          <ImagePlus />
        </Button>
        <Textarea
          ref={taRef}
          value={text}
          onChange={onChange}
          onKeyDown={onKeyDown}
          rows={1}
          disabled={!connected}
          placeholder={
            !connected
              ? '连接中…'
              : cooldownLeft > 0
                ? `发送过于频繁,请等待 ${cooldownLeft} 秒`
                : '输入消息,@ 提及他人,Enter 发送,Shift+Enter 换行'
          }
          className="max-h-40 min-h-9 flex-1 resize-none"
        />
        <Button
          variant="default"
          size="icon"
          onClick={submit}
          disabled={disabled || uploading || (!text.trim() && !image) || tooLong}
          aria-label="发送"
        >
          {uploading ? <Loader2 className="animate-spin" /> : <Send />}
        </Button>
      </div>
      <div className="mt-1 flex items-center justify-between px-1">
        <span className="text-muted-foreground text-xs">
          {cooldownLeft > 0 && `冷却中:${cooldownLeft}s`}
        </span>
        {(tooLong || text.length > maxLen * 0.8) && (
          <span className={`text-xs ${tooLong ? 'text-destructive' : 'text-muted-foreground'}`}>
            {text.length}/{maxLen}
          </span>
        )}
      </div>
    </div>
  )
}

export default MessageComposer
