import { useState, type FormEvent } from 'react'
import { Link, Navigate, useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Spinner } from '@/components/ui/spinner'
import { AuthLayout } from '@/components/layout/AuthLayout'
import { useAuth } from '@/contexts/AuthContext'
import { ApiError } from '@/lib/api'
import { toast } from '@/lib/toast'

export default function LoginPage() {
  const { login, isAuthed } = useAuth()
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)

  if (isAuthed) return <Navigate to="/" replace />

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (busy) return
    setBusy(true)
    try {
      await login(username.trim(), password)
      toast.success('登录成功')
      navigate('/')
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : '登录失败'
      toast.error('登录失败', msg)
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthLayout title="登录" subtitle="欢迎回来">
      <form onSubmit={onSubmit} className="flex flex-col gap-4">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="username">用户名</Label>
          <Input
            id="username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoComplete="username"
            required
            placeholder="请输入用户名"
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="password">密码</Label>
          <Input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
            placeholder="请输入密码"
          />
        </div>
        <Button type="submit" variant="default" disabled={busy} className="mt-2 w-full">
          {busy && <Spinner className="size-4" />}
          登录
        </Button>
      </form>
      <p className="mt-4 text-center text-muted-foreground text-sm">
        还没有账号?{' '}
        <Link to="/register" className="font-medium text-foreground underline-offset-4 hover:underline">
          立即注册
        </Link>
      </p>
    </AuthLayout>
  )
}
