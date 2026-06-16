import { Header } from '@/components/layout/Header'

// 占位:完整聊天界面(频道/私信/@提及/WebSocket)将在后端就绪后实现。
export default function ChatPage() {
  return (
    <div className="flex min-h-svh flex-col">
      <Header />
      <main className="mx-auto flex w-full max-w-7xl flex-1 items-center justify-center px-4 py-10">
        <p className="text-muted-foreground">聊天界面开发中…</p>
      </main>
    </div>
  )
}
