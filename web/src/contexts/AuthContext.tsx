import React, { createContext, useContext, useEffect, useState, useCallback } from 'react'
import { api, clearToken, getToken, setToken } from '@/lib/api'
import type { User } from '@/lib/types'

interface AuthContextType {
  user: User | null
  loading: boolean
  isAuthed: boolean
  isAdmin: boolean
  login: (username: string, password: string) => Promise<void>
  register: (
    username: string,
    password: string,
    nickname?: string,
  ) => Promise<{ pending: boolean }>
  logout: () => Promise<void>
  refresh: () => Promise<void>
  setUser: (u: User | null) => void
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    if (!getToken()) {
      setUser(null)
      setLoading(false)
      return
    }
    try {
      const u = await api.me()
      setUser(u)
    } catch {
      clearToken()
      setUser(null)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const login = useCallback(async (username: string, password: string) => {
    const res = await api.login({ username, password })
    setToken(res.token)
    setUser(res.user)
  }, [])

  const register = useCallback(
    async (username: string, password: string, nickname?: string) => {
      const res = await api.register({ username, password, nickname })
      if ('token' in res) {
        setToken(res.token)
        setUser(res.user)
        return { pending: false }
      }
      return { pending: true }
    },
    [],
  )

  const logout = useCallback(async () => {
    try {
      await api.logout()
    } catch {
      // 忽略登出接口错误,本地清除即可
    }
    clearToken()
    setUser(null)
  }, [])

  const isAuthed = !!user
  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin'

  return (
    <AuthContext.Provider
      value={{ user, loading, isAuthed, isAdmin, login, register, logout, refresh, setUser }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
