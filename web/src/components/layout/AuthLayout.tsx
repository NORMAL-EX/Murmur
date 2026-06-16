import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { MessageCircle } from 'lucide-react'
import { ThemeToggle } from '@/components/layout/ThemeToggle'
import { useSettings } from '@/contexts/SettingsContext'

export function AuthLayout({
  title,
  subtitle,
  children,
}: {
  title: string
  subtitle?: string
  children: ReactNode
}) {
  const { settings } = useSettings()
  return (
    <div className="relative flex min-h-svh flex-col items-center justify-center bg-background px-4 py-10">
      <div className="absolute top-4 right-4">
        <ThemeToggle />
      </div>
      <div className="mb-6 flex flex-col items-center gap-2">
        <Link to="/" className="flex size-12 items-center justify-center rounded-2xl bg-primary text-primary-foreground">
          <MessageCircle className="size-6" />
        </Link>
        <h1 className="font-bold text-2xl">{settings.site_title || 'Murmur'}</h1>
        {settings.site_description && (
          <p className="text-muted-foreground text-sm">{settings.site_description}</p>
        )}
      </div>
      <div className="w-full max-w-sm">
        <div className="mb-4 text-center">
          <h2 className="font-semibold text-xl">{title}</h2>
          {subtitle && <p className="mt-1 text-muted-foreground text-sm">{subtitle}</p>}
        </div>
        {children}
      </div>
    </div>
  )
}

export default AuthLayout
