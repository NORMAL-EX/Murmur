import { Outlet } from 'react-router-dom'
import { ChatProvider } from '@/contexts/ChatContext'
import { AppHeader } from '@/components/layout/AppHeader'
import { ContextMenuProvider } from '@/components/ui/context-menu'

export function MainLayout() {
  return (
    <ChatProvider>
      <ContextMenuProvider>
        <div className="flex h-svh flex-col overflow-hidden">
          <AppHeader />
          <main className="min-h-0 flex-1">
            <Outlet />
          </main>
        </div>
      </ContextMenuProvider>
    </ChatProvider>
  )
}

export default MainLayout
