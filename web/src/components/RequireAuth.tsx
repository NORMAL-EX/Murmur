import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useAuth } from '@/contexts/AuthContext'
import { Spinner } from '@/components/ui/spinner'

export function FullScreenLoader() {
  return (
    <div className="flex min-h-svh items-center justify-center">
      <Spinner className="size-6 text-muted-foreground" />
    </div>
  )
}

export function RequireAuth({ children }: { children: ReactNode }) {
  const { isAuthed, loading } = useAuth()
  const location = useLocation()
  if (loading) return <FullScreenLoader />
  if (!isAuthed) return <Navigate to="/login" replace state={{ from: location }} />
  return <>{children}</>
}

export function RequireAdmin({ children }: { children: ReactNode }) {
  const { isAdmin, loading, isAuthed } = useAuth()
  if (loading) return <FullScreenLoader />
  if (!isAuthed) return <Navigate to="/login" replace />
  if (!isAdmin) return <Navigate to="/" replace />
  return <>{children}</>
}
