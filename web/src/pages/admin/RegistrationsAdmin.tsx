import { useEffect, useState } from 'react'
import { Check, X, Inbox } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Card, CardContent } from '@/components/ui/card'
import { api, ApiError } from '@/lib/api'
import { toast } from '@/lib/toast'
import { initials, formatRelative } from '@/lib/format'
import type { User } from '@/lib/types'

export default function RegistrationsAdmin() {
  const [list, setList] = useState<User[]>([])
  const [loading, setLoading] = useState(true)

  const load = () => {
    setLoading(true)
    api.admin
      .registrations()
      .then(setList)
      .catch(() => {})
      .finally(() => setLoading(false))
  }
  useEffect(load, [])

  const act = async (fn: () => Promise<unknown>, ok: string) => {
    try {
      await fn()
      toast.success(ok)
      load()
    } catch (e) {
      toast.error('操作失败', e instanceof ApiError ? e.message : undefined)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <h1 className="font-bold text-2xl">注册审核</h1>

      {!loading && list.length === 0 && (
        <Card>
          <CardContent className="flex flex-col items-center gap-2 py-12 text-muted-foreground">
            <Inbox className="size-8 opacity-50" />
            <span>暂无待审核的注册申请</span>
          </CardContent>
        </Card>
      )}

      <div className="flex flex-col gap-2">
        {list.map((u) => (
          <Card key={u.id}>
            <CardContent className="flex items-center gap-3 p-3">
              <Avatar className="size-10">
                <AvatarFallback>{initials(u.nickname || u.username)}</AvatarFallback>
              </Avatar>
              <div className="min-w-0 flex-1">
                <div className="truncate font-medium">{u.nickname || u.username}</div>
                <div className="truncate text-muted-foreground text-xs">
                  @{u.username} · 申请于 {formatRelative(u.created_at)}
                </div>
                {u.email && (
                  <div className="truncate text-muted-foreground text-xs">📧 {u.email}</div>
                )}
              </div>
              <Button
                variant="default"
                size="sm"
                onClick={() => act(() => api.admin.approve(u.id), '已通过')}
              >
                <Check className="size-4" />
                通过
              </Button>
              <Button
                variant="destructive"
                size="sm"
                onClick={() => act(() => api.admin.reject(u.id), '已拒绝')}
              >
                <X className="size-4" />
                拒绝
              </Button>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}
