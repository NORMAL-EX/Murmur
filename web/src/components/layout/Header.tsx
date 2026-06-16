import { Link, useNavigate } from 'react-router-dom'
import { LogOut, User as UserIcon, Shield, MessageCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import {
  Menu,
  MenuTrigger,
  MenuPopup,
  MenuItem,
  MenuSeparator,
} from '@/components/ui/menu'
import { ThemeToggle } from '@/components/layout/ThemeToggle'
import { useAuth } from '@/contexts/AuthContext'
import { useSettings } from '@/contexts/SettingsContext'
import { initials } from '@/lib/format'

export function Header() {
  const { user, isAuthed, isAdmin, logout } = useAuth()
  const { settings } = useSettings()
  const navigate = useNavigate()

  const onLogout = async () => {
    await logout()
    navigate('/login')
  }

  return (
    <header className="sticky top-0 z-40 w-full border-b border-border/60 bg-background/80 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-7xl items-center gap-3 px-4">
        <Link to="/" className="group flex items-center gap-2">
          <span className="flex size-7 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <MessageCircle className="size-4" />
          </span>
          <span className="font-bold text-foreground transition-colors group-hover:text-primary">
            {settings.site_title || 'Murmur'}
          </span>
        </Link>

        <div className="flex-1" />

        <ThemeToggle />

        {isAuthed && user ? (
          <Menu>
            <MenuTrigger
              render={
                <Button variant="outline" size="icon" className="rounded-full button-header">
                  <Avatar className="size-7">
                    <AvatarImage src={user.avatar_url || undefined} alt={user.nickname} />
                    <AvatarFallback>{initials(user.nickname || user.username)}</AvatarFallback>
                  </Avatar>
                </Button>
              }
            />
            <MenuPopup className="min-w-[180px] menu-popup-animated" align="end">
              <div className="px-2 py-1.5">
                <div className="truncate font-medium text-sm">{user.nickname || user.username}</div>
                <div className="truncate text-muted-foreground text-xs">@{user.username}</div>
              </div>
              <MenuSeparator />
              <MenuItem onClick={() => navigate('/me')} className="flex items-center gap-2">
                <UserIcon className="size-4" />
                个人资料
              </MenuItem>
              {isAdmin && (
                <MenuItem onClick={() => navigate('/admin')} className="flex items-center gap-2">
                  <Shield className="size-4" />
                  管理后台
                </MenuItem>
              )}
              <MenuSeparator />
              <MenuItem onClick={onLogout} variant="destructive" className="flex items-center gap-2">
                <LogOut className="size-4" />
                退出登录
              </MenuItem>
            </MenuPopup>
          </Menu>
        ) : (
          <>
            <Button variant="outline" size="sm" render={<Link to="/login" />}>
              登录
            </Button>
            <Button variant="default" size="sm" render={<Link to="/register" />}>
              注册
            </Button>
          </>
        )}
      </div>
    </header>
  )
}

export default Header
