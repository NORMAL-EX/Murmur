// Shared domain & wire types. The Go backend mirrors these JSON shapes.

export type Role = 'super_admin' | 'admin' | 'user' | 'bot'
export type UserStatus = 'pending' | 'active' | 'banned'
export type ThemeName = 'light' | 'dark' | 'system'

export interface User {
  id: number
  username: string
  nickname: string
  avatar_url: string
  bio: string
  role: Role
  status: UserStatus
  rate_limit_per_min: number
  created_at: string
}

export interface Channel {
  id: number
  name: string
  slug: string
  description: string
  readonly: boolean
  pinned: boolean
  created_by: number
  created_at: string
}

export interface Reaction {
  emoji: string
  count: number
  user_ids: number[]
  reacted: boolean
}

export interface Message {
  id: number
  channel_id: number
  sender_id: number
  sender?: User
  content: string
  edited: boolean
  deleted: boolean
  is_bot: boolean
  mentions: number[]
  reactions: Reaction[]
  created_at: string
}

export interface DirectMessage {
  id: number
  sender_id: number
  receiver_id: number
  sender?: User
  content: string
  read_at: string | null
  created_at: string
}

export interface Conversation {
  user: User
  last_message: DirectMessage | null
  unread: number
}

export interface MentionNotice {
  id: number
  message_id: number
  channel_id: number
  mentioned_user_id: number
  read_at: string | null
  created_at: string
  message?: Message
}

export interface PublicSettings {
  site_title: string
  site_description: string
  registration_open: boolean
  registration_review: boolean
  allow_dm: boolean
  max_message_length: number
  announcement: string
  default_theme: ThemeName
  bot_name: string
  bot_avatar: string
}

export interface AuthResponse {
  token: string
  user: User
}

export interface Paginated<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  has_more: boolean
}

export interface DashboardStats {
  users: number
  pending: number
  messages: number
  online: number
  ai_calls: number
  channels: number
  recent_messages_7d: number[]
}

export interface AISettings {
  ai_enabled: boolean
  ai_base_url: string
  ai_api_key_set: boolean
  ai_model: string
  ai_system_prompt: string
  ai_temperature: number
  ai_max_tokens: number
  ai_context_char_limit: number
  ai_cooldown_seconds: number
  ai_allow_dm: boolean
  bot_name: string
  bot_avatar: string
}

export interface AuditLog {
  id: number
  actor_id: number
  actor?: User
  action: string
  target: string
  detail: string
  created_at: string
}

// ---- WebSocket envelopes ----

export type WSOutbound =
  | { type: 'chat_message'; channel_id: number; content: string; temp_id?: string }
  | { type: 'dm_message'; to: number; content: string; temp_id?: string }
  | { type: 'typing'; channel_id?: number; to?: number }
  | { type: 'read_dm'; from: number }
  | { type: 'ping' }

export type WSInbound =
  | { type: 'ready'; user_id: number; online_user_ids: number[] }
  | { type: 'chat_message'; message: Message; temp_id?: string }
  | { type: 'message_update'; message: Message }
  | { type: 'message_delete'; message_id: number; channel_id: number }
  | { type: 'dm_message'; message: DirectMessage; temp_id?: string }
  | { type: 'mention'; mention: MentionNotice }
  | { type: 'presence'; online_user_ids: number[] }
  | { type: 'typing'; user_id: number; channel_id?: number; from?: number }
  | { type: 'bot_reply'; message: Message }
  | { type: 'reaction'; message_id: number; channel_id: number; reactions: Reaction[] }
  | { type: 'pong' }
  | { type: 'error'; code: string; message: string; retry_after?: number }
