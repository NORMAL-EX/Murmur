import { Header } from '@/components/layout/Header'

// 占位:完整管理后台将在后端就绪后实现。
export default function AdminPage() {
  return (
    <div className="flex min-h-svh flex-col">
      <Header />
      <main className="mx-auto w-full max-w-7xl flex-1 px-4 py-10">
        <h1 className="font-bold text-2xl">管理后台</h1>
        <p className="mt-2 text-muted-foreground">管理后台开发中…</p>
      </main>
    </div>
  )
}
