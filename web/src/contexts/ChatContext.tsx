import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  useRef,
  useCallback,
} from 'react'
import { api } from '@/lib/api'
import { toast } from '@/lib/toast'
import { useAuth } from '@/contexts/AuthContext'
import { useChatSocket } from '@/hooks/useChatSocket'
import type {
  Channel,
  Conversation,
  DirectMessage,
  MentionNotice,
  Message,
  User,
  WSInbound,
} from '@/lib/types'

type View = { type: 'channel'; id: number } | { type: 'dm'; userId: number }
export type ReplyTarget = { id: number; author: string; snippet: string }

interface ChatContextType {
  connected: boolean
  channels: Channel[]
  members: User[]
  membersById: Map<number, User>
  onlineIds: Set<number>
  view: View | null

  // channel view
  messages: Message[]
  channelHasMore: boolean
  loadingMessages: boolean
  unreadByChannel: Record<number, number>

  // dm view
  conversations: Conversation[]
  dmMessages: DirectMessage[]
  dmHasMore: boolean

  // notifications
  mentions: MentionNotice[]
  mentionUnread: number

  typingUserIds: number[]
  cooldownUntil: number

  replyTo: ReplyTarget | null
  setReplyTo: (r: ReplyTarget | null) => void
  pendingMention: string | null
  requestMention: (username: string) => void
  clearMention: () => void

  selectChannel: (id: number) => void
  selectDm: (userId: number) => void
  loadMore: () => void
  sendMessage: (content: string, replyTo?: number) => void
  sendDm: (content: string, replyTo?: number) => void
  editMessage: (id: number, content: string) => Promise<void>
  deleteMessage: (id: number) => Promise<void>
  recallMessage: (id: number) => Promise<void>
  recallDm: (id: number) => Promise<void>
  toggleReaction: (id: number, emoji: string) => Promise<void>
  sendTyping: () => void
  refreshChannels: () => Promise<void>
  refreshConversations: () => Promise<void>
  markMentionsRead: () => Promise<void>
  totalDmUnread: number
}

const ChatContext = createContext<ChatContextType | undefined>(undefined)

function upsert(list: Message[], msg: Message): Message[] {
  const idx = list.findIndex((m) => m.id === msg.id)
  if (idx >= 0) {
    const next = list.slice()
    next[idx] = msg
    return next
  }
  return [...list, msg]
}

export const ChatProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { user } = useAuth()
  const myId = user?.id ?? 0

  const [channels, setChannels] = useState<Channel[]>([])
  const [members, setMembers] = useState<User[]>([])
  const [onlineIds, setOnlineIds] = useState<Set<number>>(new Set())
  const [view, setView] = useState<View | null>(null)

  const [messages, setMessages] = useState<Message[]>([])
  const [channelHasMore, setChannelHasMore] = useState(false)
  const [loadingMessages, setLoadingMessages] = useState(false)
  const [unreadByChannel, setUnreadByChannel] = useState<Record<number, number>>({})

  const [conversations, setConversations] = useState<Conversation[]>([])
  const [dmMessages, setDmMessages] = useState<DirectMessage[]>([])
  const [dmHasMore, setDmHasMore] = useState(false)

  const [mentions, setMentions] = useState<MentionNotice[]>([])
  const [typingUserIds, setTypingUserIds] = useState<number[]>([])
  const [cooldownUntil, setCooldownUntil] = useState(0)
  const [replyTo, setReplyTo] = useState<ReplyTarget | null>(null)
  const [pendingMention, setPendingMention] = useState<string | null>(null)

  const viewRef = useRef<View | null>(null)
  viewRef.current = view
  const requestKey = useRef(0)
  const typingTimers = useRef<Record<number, number>>({})
  const lastTypingSent = useRef(0)

  const membersById = new Map(members.map((m) => [m.id, m]))
  const mentionUnread = mentions.filter((m) => !m.read_at).length
  const totalDmUnread = conversations.reduce((sum, c) => sum + c.unread, 0)

  // ---- socket event handling ----
  const handleEvent = useCallback(
    (e: WSInbound) => {
      switch (e.type) {
        case 'ready':
        case 'presence':
          setOnlineIds(new Set(e.online_user_ids))
          break
        case 'chat_message':
        case 'bot_reply': {
          const m = e.message
          const v = viewRef.current
          if (v?.type === 'channel' && v.id === m.channel_id) {
            setMessages((prev) => upsert(prev, m))
          } else {
            setUnreadByChannel((prev) => ({
              ...prev,
              [m.channel_id]: (prev[m.channel_id] || 0) + 1,
            }))
          }
          break
        }
        case 'message_update': {
          const m = e.message
          const v = viewRef.current
          if (v?.type === 'channel' && v.id === m.channel_id) {
            setMessages((prev) => prev.map((x) => (x.id === m.id ? m : x)))
          }
          break
        }
        case 'message_delete': {
          const v = viewRef.current
          if (v?.type === 'channel' && v.id === e.channel_id) {
            setMessages((prev) =>
              prev.map((x) =>
                x.id === e.message_id ? { ...x, deleted: true, content: '' } : x,
              ),
            )
          }
          break
        }
        case 'reaction': {
          const v = viewRef.current
          if (v?.type === 'channel' && v.id === e.channel_id) {
            setMessages((prev) =>
              prev.map((x) => (x.id === e.message_id ? { ...x, reactions: e.reactions } : x)),
            )
          }
          break
        }
        case 'message_recalled': {
          const v = viewRef.current
          if (v?.type === 'channel' && v.id === e.channel_id) {
            setMessages((prev) =>
              prev.map((x) =>
                x.id === e.message_id
                  ? { ...x, recalled: true, content: '', recalled_by: e.recalled_by }
                  : x,
              ),
            )
          }
          break
        }
        case 'dm_recalled': {
          const partner = e.sender_id === myId ? e.receiver_id : e.sender_id
          const v = viewRef.current
          if (v?.type === 'dm' && v.userId === partner) {
            setDmMessages((prev) =>
              prev.map((x) =>
                x.id === e.message_id
                  ? { ...x, recalled: true, content: '', recalled_by: e.recalled_by }
                  : x,
              ),
            )
          }
          void refreshConversationsRef.current()
          break
        }
        case 'dm_message': {
          const m = e.message
          const partner = m.sender_id === myId ? m.receiver_id : m.sender_id
          const v = viewRef.current
          if (v?.type === 'dm' && v.userId === partner) {
            setDmMessages((prev) =>
              prev.some((x) => x.id === m.id) ? prev : [...prev, m],
            )
            if (m.sender_id === partner) void api.markDmRead(partner)
          }
          void refreshConversationsRef.current()
          if (m.sender_id !== myId && !(v?.type === 'dm' && v.userId === partner)) {
            const who = membersByIdRef.current.get(m.sender_id)
            toast.message('新私信', `${who?.nickname || '有人'} 给你发来私信`)
          }
          break
        }
        case 'mention': {
          setMentions((prev) => [e.mention, ...prev.filter((x) => x.id !== e.mention.id)])
          toast.info('有人提到了你', e.mention.message?.content?.slice(0, 60))
          break
        }
        case 'typing': {
          const uid = e.user_id
          if (uid === myId) break
          const v = viewRef.current
          const relevant =
            (e.channel_id && v?.type === 'channel' && v.id === e.channel_id) ||
            (e.from && v?.type === 'dm' && v.userId === e.from)
          if (!relevant) break
          setTypingUserIds((prev) => (prev.includes(uid) ? prev : [...prev, uid]))
          window.clearTimeout(typingTimers.current[uid])
          typingTimers.current[uid] = window.setTimeout(() => {
            setTypingUserIds((prev) => prev.filter((x) => x !== uid))
          }, 3500)
          break
        }
        case 'error': {
          if (e.code === 'rate_limited' && e.retry_after) {
            setCooldownUntil(Date.now() + e.retry_after * 1000)
          }
          toast.error('发送失败', e.message)
          break
        }
      }
    },
    [myId],
  )

  const resyncRef = useRef<() => void>(() => {})
  const { connected, send } = useChatSocket({
    onEvent: handleEvent,
    onReconnect: () => resyncRef.current(),
  })

  // refs to call latest versions inside the socket handler
  const refreshConversationsRef = useRef<() => Promise<void>>(async () => {})
  const membersByIdRef = useRef(membersById)
  membersByIdRef.current = membersById

  const refreshChannels = useCallback(async () => {
    try {
      setChannels(await api.channels())
    } catch {
      /* ignore */
    }
  }, [])

  const refreshConversations = useCallback(async () => {
    try {
      setConversations(await api.conversations())
    } catch {
      /* ignore */
    }
  }, [])
  refreshConversationsRef.current = refreshConversations

  const refreshMembers = useCallback(async () => {
    try {
      setMembers(await api.members())
    } catch {
      /* ignore */
    }
  }, [])

  const refreshMentions = useCallback(async () => {
    try {
      setMentions(await api.mentions())
    } catch {
      /* ignore */
    }
  }, [])

  // Re-sync state after a dropped connection is restored (catch up missed data).
  const resync = useCallback(() => {
    void refreshChannels()
    void refreshMembers()
    void refreshConversations()
    void refreshMentions()
    const v = viewRef.current
    if (v?.type === 'channel') {
      api
        .channelMessages(v.id)
        .then((res) => {
          setMessages(res.items)
          setChannelHasMore(res.has_more)
        })
        .catch(() => {})
    } else if (v?.type === 'dm') {
      api
        .dmMessages(v.userId)
        .then((res) => {
          setDmMessages(res.items)
          setDmHasMore(res.has_more)
        })
        .catch(() => {})
    }
    toast.success('已重新连接')
  }, [refreshChannels, refreshMembers, refreshConversations, refreshMentions])
  resyncRef.current = resync

  // bootstrap once authenticated
  useEffect(() => {
    if (!myId) return
    void refreshChannels()
    void refreshMembers()
    void refreshConversations()
    void refreshMentions()
  }, [myId, refreshChannels, refreshMembers, refreshConversations, refreshMentions])

  // pick a default channel
  useEffect(() => {
    if (!view && channels.length > 0) {
      setView({ type: 'channel', id: channels[0].id })
    }
  }, [channels, view])

  // load messages when the channel view changes
  useEffect(() => {
    if (view?.type !== 'channel') return
    const id = view.id
    const key = ++requestKey.current
    setLoadingMessages(true)
    setTypingUserIds([])
    api
      .channelMessages(id)
      .then((res) => {
        if (key !== requestKey.current) return
        setMessages(res.items)
        setChannelHasMore(res.has_more)
        setUnreadByChannel((prev) => ({ ...prev, [id]: 0 }))
      })
      .catch(() => {})
      .finally(() => {
        if (key === requestKey.current) setLoadingMessages(false)
      })
  }, [view])

  // load dm messages when the dm view changes
  useEffect(() => {
    if (view?.type !== 'dm') return
    const uid = view.userId
    const key = ++requestKey.current
    setTypingUserIds([])
    api
      .dmMessages(uid)
      .then((res) => {
        if (key !== requestKey.current) return
        setDmMessages(res.items)
        setDmHasMore(res.has_more)
        void refreshConversations()
      })
      .catch(() => {})
  }, [view, refreshConversations])

  const selectChannel = useCallback((id: number) => {
    setReplyTo(null)
    setView({ type: 'channel', id })
  }, [])
  const selectDm = useCallback((userId: number) => {
    setReplyTo(null)
    setView({ type: 'dm', userId })
  }, [])
  const requestMention = useCallback((username: string) => setPendingMention(username), [])
  const clearMention = useCallback(() => setPendingMention(null), [])

  const loadMore = useCallback(() => {
    const v = viewRef.current
    if (!v) return
    if (v.type === 'channel') {
      if (!channelHasMore || messages.length === 0) return
      const before = messages[0].id
      api.channelMessages(v.id, before).then((res) => {
        setMessages((prev) => [...res.items, ...prev])
        setChannelHasMore(res.has_more)
      })
    } else {
      if (!dmHasMore || dmMessages.length === 0) return
      const before = dmMessages[0].id
      api.dmMessages(v.userId, before).then((res) => {
        setDmMessages((prev) => [...res.items, ...prev])
        setDmHasMore(res.has_more)
      })
    }
  }, [channelHasMore, messages, dmHasMore, dmMessages])

  const sendMessage = useCallback(
    (content: string, replyTo?: number) => {
      const v = viewRef.current
      if (v?.type !== 'channel') return
      if (!send({ type: 'chat_message', channel_id: v.id, content, reply_to: replyTo })) {
        toast.error('连接已断开', '正在重连,请稍候')
      }
    },
    [send],
  )

  const sendDm = useCallback(
    (content: string, replyTo?: number) => {
      const v = viewRef.current
      if (v?.type !== 'dm') return
      if (!send({ type: 'dm_message', to: v.userId, content, reply_to: replyTo })) {
        toast.error('连接已断开', '正在重连,请稍候')
      }
    },
    [send],
  )

  const editMessage = useCallback(async (id: number, content: string) => {
    await api.editMessage(id, content)
  }, [])

  const deleteMessage = useCallback(async (id: number) => {
    await api.deleteMessage(id)
  }, [])

  const recallMessage = useCallback(async (id: number) => {
    await api.recallMessage(id)
  }, [])

  const recallDm = useCallback(async (id: number) => {
    await api.recallDm(id)
  }, [])

  const toggleReaction = useCallback(async (id: number, emoji: string) => {
    await api.toggleReaction(id, emoji)
  }, [])

  const sendTyping = useCallback(() => {
    const v = viewRef.current
    if (!v) return
    const now = Date.now()
    if (now - lastTypingSent.current < 1500) return
    lastTypingSent.current = now
    if (v.type === 'channel') send({ type: 'typing', channel_id: v.id })
    else send({ type: 'typing', to: v.userId })
  }, [send])

  const markMentionsRead = useCallback(async () => {
    await api.readAllMentions()
    setMentions((prev) => prev.map((m) => ({ ...m, read_at: m.read_at || new Date().toISOString() })))
  }, [])

  const value: ChatContextType = {
    connected,
    channels,
    members,
    membersById,
    onlineIds,
    view,
    messages,
    channelHasMore,
    loadingMessages,
    unreadByChannel,
    conversations,
    dmMessages,
    dmHasMore,
    mentions,
    mentionUnread,
    typingUserIds,
    cooldownUntil,
    replyTo,
    setReplyTo,
    pendingMention,
    requestMention,
    clearMention,
    selectChannel,
    selectDm,
    loadMore,
    sendMessage,
    sendDm,
    editMessage,
    deleteMessage,
    recallMessage,
    recallDm,
    toggleReaction,
    sendTyping,
    refreshChannels,
    refreshConversations,
    markMentionsRead,
    totalDmUnread,
  }

  return <ChatContext.Provider value={value}>{children}</ChatContext.Provider>
}

export function useChat() {
  const ctx = useContext(ChatContext)
  if (!ctx) throw new Error('useChat must be used within ChatProvider')
  return ctx
}
