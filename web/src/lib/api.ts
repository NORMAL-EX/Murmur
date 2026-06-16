import type {
  AISettings,
  AuditLog,
  AuthResponse,
  Channel,
  Conversation,
  DashboardStats,
  DirectMessage,
  MentionNotice,
  Message,
  Paginated,
  PublicSettings,
  Role,
  User,
  UserStatus,
} from './types'

const TOKEN_KEY = 'murmur_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}
export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}
export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

export class ApiError extends Error {
  code: string
  status: number
  retryAfter?: number
  constructor(status: number, code: string, message: string, retryAfter?: number) {
    super(message)
    this.status = status
    this.code = code
    this.retryAfter = retryAfter
  }
}

type Json = Record<string, unknown>

async function request<T>(
  method: string,
  path: string,
  body?: Json | FormData,
): Promise<T> {
  const headers: Record<string, string> = {}
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`

  let payload: BodyInit | undefined
  if (body instanceof FormData) {
    payload = body
  } else if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    payload = JSON.stringify(body)
  }

  const res = await fetch(`/api${path}`, { method, headers, body: payload })

  if (res.status === 204) return undefined as T

  let data: unknown = null
  const text = await res.text()
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = text
    }
  }

  if (!res.ok) {
    const d = (data ?? {}) as {
      code?: string
      error?: string
      message?: string
      retry_after?: number
    }
    if (res.status === 401) clearToken()
    throw new ApiError(
      res.status,
      d.code || 'error',
      d.error || d.message || `请求失败 (${res.status})`,
      d.retry_after,
    )
  }
  return data as T
}

export const api = {
  // auth
  register: (b: { username: string; password: string; nickname?: string }) =>
    request<AuthResponse | { pending: true }>('POST', '/auth/register', b),
  login: (b: { username: string; password: string }) =>
    request<AuthResponse>('POST', '/auth/login', b),
  logout: () => request<void>('POST', '/auth/logout'),
  me: () => request<User>('GET', '/me'),
  updateMe: (b: { nickname?: string; bio?: string; password?: string }) =>
    request<User>('PATCH', '/me', b),
  uploadAvatar: (file: File) => {
    const fd = new FormData()
    fd.append('avatar', file)
    return request<{ avatar_url: string }>('POST', '/me/avatar', fd)
  },

  // settings (public)
  settings: () => request<PublicSettings>('GET', '/settings'),

  // channels
  channels: () => request<Channel[]>('GET', '/channels'),
  createChannel: (b: { name: string; description?: string }) =>
    request<Channel>('POST', '/channels', b),
  updateChannel: (
    id: number,
    b: { name?: string; description?: string; readonly?: boolean; pinned?: boolean },
  ) => request<Channel>('PATCH', `/channels/${id}`, b),
  deleteChannel: (id: number) => request<void>('DELETE', `/channels/${id}`),
  channelMessages: (id: number, beforeId?: number, limit = 30) =>
    request<Paginated<Message>>(
      'GET',
      `/channels/${id}/messages?limit=${limit}${beforeId ? `&before=${beforeId}` : ''}`,
    ),

  // messages
  editMessage: (id: number, content: string) =>
    request<Message>('PATCH', `/messages/${id}`, { content }),
  deleteMessage: (id: number) => request<void>('DELETE', `/messages/${id}`),
  toggleReaction: (id: number, emoji: string) =>
    request<{ message_id: number; reactions: Message['reactions'] }>(
      'POST',
      `/messages/${id}/reactions`,
      { emoji },
    ),
  searchMessages: (channelId: number, q: string) =>
    request<Message[]>('GET', `/channels/${channelId}/search?q=${encodeURIComponent(q)}`),

  // members (for mention autocomplete & profiles)
  members: (q = '') => request<User[]>('GET', `/users?q=${encodeURIComponent(q)}`),
  user: (id: number) => request<User>('GET', `/users/${id}`),

  // dm
  conversations: () => request<Conversation[]>('GET', '/dm/conversations'),
  dmMessages: (userId: number, beforeId?: number, limit = 30) =>
    request<Paginated<DirectMessage>>(
      'GET',
      `/dm/${userId}/messages?limit=${limit}${beforeId ? `&before=${beforeId}` : ''}`,
    ),
  sendDm: (userId: number, content: string) =>
    request<DirectMessage>('POST', `/dm/${userId}`, { content }),
  markDmRead: (userId: number) => request<void>('POST', `/dm/${userId}/read`),

  // mentions
  mentions: () => request<MentionNotice[]>('GET', '/mentions'),
  readMention: (id: number) => request<void>('POST', `/mentions/${id}/read`),
  readAllMentions: () => request<void>('POST', '/mentions/read-all'),

  // ---- admin ----
  admin: {
    stats: () => request<DashboardStats>('GET', '/admin/stats'),
    users: (q = '', page = 1, pageSize = 20) =>
      request<Paginated<User>>(
        'GET',
        `/admin/users?q=${encodeURIComponent(q)}&page=${page}&page_size=${pageSize}`,
      ),
    updateUser: (
      id: number,
      b: {
        status?: UserStatus
        role?: Role
        rate_limit_per_min?: number
        nickname?: string
      },
    ) => request<User>('PATCH', `/admin/users/${id}`, b),
    deleteUser: (id: number) => request<void>('DELETE', `/admin/users/${id}`),
    registrations: () => request<User[]>('GET', '/admin/registrations'),
    approve: (id: number) => request<void>('POST', `/admin/registrations/${id}/approve`),
    reject: (id: number) => request<void>('POST', `/admin/registrations/${id}/reject`),
    getSettings: () => request<Record<string, string>>('GET', '/admin/settings'),
    putSettings: (b: Record<string, string | number | boolean>) =>
      request<void>('PUT', '/admin/settings', b),
    getAISettings: () => request<AISettings>('GET', '/admin/ai'),
    testAI: () =>
      request<{ ok: boolean; message: string; reply?: string }>('POST', '/admin/ai/test'),
    auditLogs: (page = 1) =>
      request<Paginated<AuditLog>>('GET', `/admin/audit?page=${page}`),
  },
}
