import { Link } from 'react-router-dom'
import { MessageCircle, Wifi, WifiOff } from 'lucide-react'
import { ThemeToggle } from '@/components/layout/ThemeToggle'
import { UserMenu } from '@/components/layout/UserMenu'
import { MentionsBell } from '@/components/chat/MentionsBell'
import { useSettings } from '@/contexts/SettingsContext'
import { useChat } from '@/contexts/ChatContext'
import { Tooltip, TooltipTrigger, TooltipPopup } from '@/components/ui/tooltip'

export function AppHeader() {
  const { settings } = useSettings()
  const { connected } = useChat()

  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-b border-border bg-background px-4">
      <Link to="/" className="group flex items-center gap-2">
        <span className="flex size-7 items-center justify-center rounded-lg bg-primary text-primary-foreground">
          <MessageCircle className="size-4" />
        </span>
        <span className="font-bold text-foreground transition-colors group-hover:text-primary">
          {settings.site_title || 'Murmur'}
        </span>
      </Link>

      <div className="flex-1" />

      <Tooltip>
        <TooltipTrigger
          render={
            <span
              className={`flex size-7 items-center justify-center ${
                connected ? 'text-foreground' : 'text-muted-foreground'
              }`}
            >
              {connected ? <Wifi className="size-4" /> : <WifiOff className="size-4" />}
            </span>
          }
        />
        <TooltipPopup>{connected ? '已连接' : '连接中…'}</TooltipPopup>
      </Tooltip>

      <MentionsBell />
      <ThemeToggle />
      <UserMenu />
    </header>
  )
}

export default AppHeader
