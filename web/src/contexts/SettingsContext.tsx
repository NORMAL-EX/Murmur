import React, { createContext, useContext, useEffect, useState, useCallback } from 'react'
import { api } from '@/lib/api'
import type { PublicSettings } from '@/lib/types'

const DEFAULTS: PublicSettings = {
  site_title: 'Murmur',
  site_description: '实时聊天室',
  registration_open: true,
  registration_review: false,
  allow_dm: true,
  max_message_length: 2000,
  announcement: '',
  default_theme: 'system',
  bot_name: 'Murmur Bot',
  bot_avatar: '',
}

interface SettingsContextType {
  settings: PublicSettings
  refresh: () => Promise<void>
}

const SettingsContext = createContext<SettingsContextType | undefined>(undefined)

export const SettingsProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [settings, setSettings] = useState<PublicSettings>(DEFAULTS)

  const refresh = useCallback(async () => {
    try {
      const s = await api.settings()
      setSettings({ ...DEFAULTS, ...s })
    } catch {
      // 后端不可用时使用默认值
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  // 站点标题即时生效:写入 <title>
  useEffect(() => {
    document.title = settings.site_title || 'Murmur'
  }, [settings.site_title])

  return (
    <SettingsContext.Provider value={{ settings, refresh }}>
      {children}
    </SettingsContext.Provider>
  )
}

export function useSettings() {
  const ctx = useContext(SettingsContext)
  if (!ctx) throw new Error('useSettings must be used within SettingsProvider')
  return ctx
}
