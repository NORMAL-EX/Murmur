import { useCallback, useEffect, useState, type ReactNode } from 'react'
import {
  MoreHorizontal,
  Ban,
  ShieldCheck,
  ShieldOff,
  Gauge,
  Trash2,
  Undo2,
  Pencil,
  MicOff,
  Mic,
  UserPlus,
} from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '@/components/ui/table'
import { Menu, MenuTrigger, MenuPopup, MenuItem, MenuSeparator } from '@/components/ui/menu'
import {
  Dialog,
  DialogPopup,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogPanel,
  DialogFooter,
  DialogClose,
} from '@/components/ui/dialog'
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectPopup,
  SelectItem,
} from '@/components/ui/select'
import {
  NumberField,
  NumberFieldGroup,
  NumberFieldDecrement,
  NumberFieldInput,
  NumberFieldIncrement,
} from '@/components/ui/number-field'
import {
  AlertDialog,
  AlertDialogPopup,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogClose,
} from '@/components/ui/alert-dialog'
import { useAuth } from '@/contexts/AuthContext'
import { api, ApiError } from '@/lib/api'
import { toast } from '@/lib/toast'
import { initials } from '@/lib/format'
import type { Paginated, Role, User } from '@/lib/types'

const ROLE_LABEL: Record<Role, string> = {
  super_admin: '超级管理员',
  admin: '管理员',
  user: '用户',
  bot: '机器人',
}

function rateText(v: number) {
  if (v < 0) return '默认'
  if (v === 0) return '不限'
  return `${v}/分`
}

function isMuted(u: User) {
  return !!u.muted_until && new Date(u.muted_until).getTime() > Date.now()
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-muted-foreground text-sm">{label}</span>
      {children}
    </div>
  )
}

export default function UsersAdmin() {
  const { user: me } = useAuth()
  const isSuper = me?.role === 'super_admin'

  const [q, setQ] = useState('')
  const [page, setPage] = useState(1)
  const [data, setData] = useState<Paginated<User> | null>(null)
  const [loading, setLoading] = useState(false)

  const [rateUser, setRateUser] = useState<User | null>(null)
  const [deleteUser, setDeleteUser] = useState<User | null>(null)
  const [editUser, setEditUser] = useState<User | null>(null)
  const [muteUser, setMuteUser] = useState<User | null>(null)
  const [createOpen, setCreateOpen] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setData(await api.admin.users(q, page))
    } catch {
      toast.error('加载失败')
    } finally {
      setLoading(false)
    }
  }, [q, page])

  useEffect(() => {
    const t = setTimeout(load, 250)
    return () => clearTimeout(t)
  }, [load])

  const act = async (fn: () => Promise<unknown>, ok = '已更新') => {
    try {
      await fn()
      toast.success(ok)
      load()
    } catch (e) {
      toast.error('操作失败', e instanceof ApiError ? e.message : undefined)
    }
  }

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.page_size)) : 1

  return (
    <div className="flex flex-col gap-4">
      <h1 className="font-bold text-2xl">用户管理</h1>

      <div className="flex items-center gap-2">
        <Input
          value={q}
          onChange={(e) => {
            setPage(1)
            setQ(e.target.value)
          }}
          placeholder="搜索用户名或昵称"
          className="max-w-xs"
        />
        <Button variant="default" className="gap-1" onClick={() => setCreateOpen(true)}>
          <UserPlus className="size-4" />
          新建用户
        </Button>
      </div>

      <div className="overflow-x-auto rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>用户</TableHead>
              <TableHead>角色</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>频率</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data?.items.map((u) => {
              const locked = u.role === 'super_admin' || (!isSuper && u.role === 'admin')
              return (
                <TableRow key={u.id}>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <Avatar className="size-7">
                        <AvatarImage src={u.avatar_url || undefined} />
                        <AvatarFallback>{initials(u.nickname || u.username)}</AvatarFallback>
                      </Avatar>
                      <div className="min-w-0">
                        <div className="truncate font-medium text-sm">{u.nickname || u.username}</div>
                        <div className="truncate text-muted-foreground text-xs">@{u.username}</div>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant={u.role === 'user' ? 'outline' : 'secondary'}>
                      {ROLE_LABEL[u.role]}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {u.status === 'active' && <Badge variant="outline">正常</Badge>}
                    {u.status === 'banned' && <Badge variant="destructive">已封禁</Badge>}
                    {u.status === 'pending' && <Badge variant="secondary">待审核</Badge>}
                    {isMuted(u) && (
                      <Badge variant="secondary" className="ml-1">
                        禁言中
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell className="tabular-nums text-sm">{rateText(u.rate_limit_per_min)}</TableCell>
                  <TableCell className="text-right">
                    <Menu>
                      <MenuTrigger
                        render={
                          <Button variant="outline" size="icon-sm" disabled={locked} aria-label="操作">
                            <MoreHorizontal />
                          </Button>
                        }
                      />
                      <MenuPopup className="min-w-[170px] menu-popup-animated" align="end">
                        <MenuItem className="flex items-center gap-2" onClick={() => setEditUser(u)}>
                          <Pencil className="size-4" />
                          编辑资料
                        </MenuItem>
                        {u.status === 'banned' ? (
                          <MenuItem
                            className="flex items-center gap-2"
                            onClick={() => act(() => api.admin.updateUser(u.id, { status: 'active' }))}
                          >
                            <Undo2 className="size-4" />
                            解除封禁
                          </MenuItem>
                        ) : (
                          <MenuItem
                            className="flex items-center gap-2"
                            onClick={() => act(() => api.admin.updateUser(u.id, { status: 'banned' }))}
                          >
                            <Ban className="size-4" />
                            封禁
                          </MenuItem>
                        )}
                        <MenuItem className="flex items-center gap-2" onClick={() => setRateUser(u)}>
                          <Gauge className="size-4" />
                          调整频率
                        </MenuItem>
                        {isMuted(u) ? (
                          <MenuItem
                            className="flex items-center gap-2"
                            onClick={() => act(() => api.admin.updateUser(u.id, { mute_minutes: 0 }))}
                          >
                            <Mic className="size-4" />
                            解除禁言
                          </MenuItem>
                        ) : (
                          <MenuItem className="flex items-center gap-2" onClick={() => setMuteUser(u)}>
                            <MicOff className="size-4" />
                            禁言
                          </MenuItem>
                        )}
                        {isSuper && u.role === 'user' && (
                          <MenuItem
                            className="flex items-center gap-2"
                            onClick={() => act(() => api.admin.updateUser(u.id, { role: 'admin' }))}
                          >
                            <ShieldCheck className="size-4" />
                            设为管理员
                          </MenuItem>
                        )}
                        {isSuper && u.role === 'admin' && (
                          <MenuItem
                            className="flex items-center gap-2"
                            onClick={() => act(() => api.admin.updateUser(u.id, { role: 'user' }))}
                          >
                            <ShieldOff className="size-4" />
                            取消管理员
                          </MenuItem>
                        )}
                        {isSuper && (
                          <>
                            <MenuSeparator />
                            <MenuItem
                              variant="destructive"
                              className="flex items-center gap-2"
                              onClick={() => setDeleteUser(u)}
                            >
                              <Trash2 className="size-4" />
                              删除用户
                            </MenuItem>
                          </>
                        )}
                      </MenuPopup>
                    </Menu>
                  </TableCell>
                </TableRow>
              )
            })}
            {!loading && data?.items.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                  没有找到用户
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <div className="flex items-center justify-between">
        <span className="text-muted-foreground text-sm">共 {data?.total ?? 0} 名用户</span>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
            上一页
          </Button>
          <span className="text-sm tabular-nums">
            {page} / {totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            下一页
          </Button>
        </div>
      </div>

      {createOpen && (
        <CreateUserDialog
          isSuper={isSuper}
          onClose={() => setCreateOpen(false)}
          onSaved={() => {
            setCreateOpen(false)
            load()
          }}
        />
      )}

      {rateUser && (
        <RateDialog
          user={rateUser}
          onClose={() => setRateUser(null)}
          onSaved={() => {
            setRateUser(null)
            load()
          }}
        />
      )}

      {editUser && (
        <EditUserDialog
          user={editUser}
          onClose={() => setEditUser(null)}
          onSaved={() => {
            setEditUser(null)
            load()
          }}
        />
      )}

      {muteUser && (
        <MuteDialog
          user={muteUser}
          onClose={() => setMuteUser(null)}
          onSaved={() => {
            setMuteUser(null)
            load()
          }}
        />
      )}

      <AlertDialog open={!!deleteUser} onOpenChange={(o) => !o && setDeleteUser(null)}>
        <AlertDialogPopup>
          <AlertDialogHeader>
            <AlertDialogTitle>删除用户</AlertDialogTitle>
            <AlertDialogDescription>
              确定要删除 @{deleteUser?.username} 吗？此操作不可撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogClose render={<Button variant="outline" />}>取消</AlertDialogClose>
            <AlertDialogClose
              render={<Button variant="destructive" />}
              onClick={() =>
                deleteUser && act(() => api.admin.deleteUser(deleteUser.id), '已删除')
              }
            >
              删除
            </AlertDialogClose>
          </AlertDialogFooter>
        </AlertDialogPopup>
      </AlertDialog>
    </div>
  )
}

function CreateUserDialog({
  isSuper,
  onClose,
  onSaved,
}: {
  isSuper: boolean
  onClose: () => void
  onSaved: () => void
}) {
  const [username, setUsername] = useState('')
  const [nickname, setNickname] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<'user' | 'admin'>('user')
  const [saving, setSaving] = useState(false)

  const save = async () => {
    if (username.trim().length < 3) {
      toast.error('用户名至少 3 位')
      return
    }
    if (password.length < 6) {
      toast.error('密码至少 6 位')
      return
    }
    setSaving(true)
    try {
      await api.admin.createUser({
        username: username.trim(),
        password,
        nickname: nickname.trim() || undefined,
        email: email.trim() || undefined,
        role,
      })
      toast.success('用户已创建')
      onSaved()
    } catch (e) {
      toast.error('创建失败', e instanceof ApiError ? e.message : undefined)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogPopup className="max-w-sm">
        <DialogHeader>
          <DialogTitle>新建用户</DialogTitle>
          <DialogDescription>直接创建一个已激活账号（无需审核 / 邮箱验证）。</DialogDescription>
        </DialogHeader>
        <DialogPanel className="flex flex-col gap-3">
          <Field label="用户名">
            <Input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="字母、数字、下划线" />
          </Field>
          <Field label="昵称（可选）">
            <Input value={nickname} onChange={(e) => setNickname(e.target.value)} />
          </Field>
          <Field label="邮箱（可选）">
            <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
          </Field>
          <Field label="密码">
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="至少 6 位"
            />
          </Field>
          {isSuper && (
            <Field label="角色">
              <Select value={role} onValueChange={(v) => setRole(v as 'user' | 'admin')}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectPopup>
                  <SelectItem value="user">用户</SelectItem>
                  <SelectItem value="admin">管理员</SelectItem>
                </SelectPopup>
              </Select>
            </Field>
          )}
        </DialogPanel>
        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>取消</DialogClose>
          <Button variant="default" onClick={save} disabled={saving}>
            创建
          </Button>
        </DialogFooter>
      </DialogPopup>
    </Dialog>
  )
}

function EditUserDialog({
  user,
  onClose,
  onSaved,
}: {
  user: User
  onClose: () => void
  onSaved: () => void
}) {
  const [username, setUsername] = useState(user.username)
  const [nickname, setNickname] = useState(user.nickname)
  const [email, setEmail] = useState(user.email || '')
  const [avatar, setAvatar] = useState(user.avatar_url || '')
  const [password, setPassword] = useState('')
  const [saving, setSaving] = useState(false)

  const save = async () => {
    setSaving(true)
    try {
      await api.admin.updateUser(user.id, {
        username: username.trim(),
        nickname: nickname.trim(),
        email: email.trim(),
        avatar_url: avatar.trim(),
        ...(password ? { password } : {}),
      })
      toast.success('已保存')
      onSaved()
    } catch (e) {
      toast.error('保存失败', e instanceof ApiError ? e.message : undefined)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogPopup className="max-w-sm">
        <DialogHeader>
          <DialogTitle>编辑用户 · @{user.username}</DialogTitle>
          <DialogDescription>修改该用户资料，留空密码则不更改。</DialogDescription>
        </DialogHeader>
        <DialogPanel className="flex flex-col gap-3">
          <Field label="用户名">
            <Input value={username} onChange={(e) => setUsername(e.target.value)} />
          </Field>
          <Field label="昵称">
            <Input value={nickname} onChange={(e) => setNickname(e.target.value)} />
          </Field>
          <Field label="邮箱">
            <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
          </Field>
          <Field label="头像 URL">
            <Input value={avatar} onChange={(e) => setAvatar(e.target.value)} placeholder="/uploads/... 或 https://" />
          </Field>
          <Field label="重置密码（留空不改）">
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="至少 6 位"
            />
          </Field>
        </DialogPanel>
        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>取消</DialogClose>
          <Button variant="default" onClick={save} disabled={saving}>
            保存
          </Button>
        </DialogFooter>
      </DialogPopup>
    </Dialog>
  )
}

const MUTE_OPTIONS = [
  { label: '10 分钟', minutes: 10 },
  { label: '1 小时', minutes: 60 },
  { label: '1 天', minutes: 1440 },
  { label: '7 天', minutes: 10080 },
  { label: '永久', minutes: -1 },
]

function MuteDialog({
  user,
  onClose,
  onSaved,
}: {
  user: User
  onClose: () => void
  onSaved: () => void
}) {
  const [minutes, setMinutes] = useState('60')
  const [saving, setSaving] = useState(false)

  const save = async () => {
    setSaving(true)
    try {
      await api.admin.updateUser(user.id, { mute_minutes: Number(minutes) })
      toast.success('已禁言')
      onSaved()
    } catch (e) {
      toast.error('操作失败', e instanceof ApiError ? e.message : undefined)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogPopup className="max-w-sm">
        <DialogHeader>
          <DialogTitle>禁言 · @{user.username}</DialogTitle>
          <DialogDescription>禁言期间该用户可浏览但不能发送消息。</DialogDescription>
        </DialogHeader>
        <DialogPanel>
          <Select value={minutes} onValueChange={(v) => setMinutes(v as string)}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectPopup>
              {MUTE_OPTIONS.map((o) => (
                <SelectItem key={o.minutes} value={String(o.minutes)}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectPopup>
          </Select>
        </DialogPanel>
        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>取消</DialogClose>
          <Button variant="default" onClick={save} disabled={saving}>
            确定禁言
          </Button>
        </DialogFooter>
      </DialogPopup>
    </Dialog>
  )
}

function RateDialog({
  user,
  onClose,
  onSaved,
}: {
  user: User
  onClose: () => void
  onSaved: () => void
}) {
  const initialMode = user.rate_limit_per_min < 0 ? 'inherit' : user.rate_limit_per_min === 0 ? 'unlimited' : 'custom'
  const [mode, setMode] = useState(initialMode)
  const [value, setValue] = useState(user.rate_limit_per_min > 0 ? user.rate_limit_per_min : 10)
  const [saving, setSaving] = useState(false)

  const save = async () => {
    const rate = mode === 'inherit' ? -1 : mode === 'unlimited' ? 0 : Math.max(1, value)
    setSaving(true)
    try {
      await api.admin.updateUser(user.id, { rate_limit_per_min: rate })
      toast.success('频率已更新')
      onSaved()
    } catch (e) {
      toast.error('保存失败', e instanceof ApiError ? e.message : undefined)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogPopup className="max-w-sm">
        <DialogHeader>
          <DialogTitle>发送频率 · @{user.username}</DialogTitle>
          <DialogDescription>限制该用户在单位时间内可发送的消息数量。</DialogDescription>
        </DialogHeader>
        <DialogPanel className="flex flex-col gap-4">
          <Select value={mode} onValueChange={(v) => setMode(v as string)}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectPopup>
              <SelectItem value="inherit">继承全局默认</SelectItem>
              <SelectItem value="unlimited">不限制</SelectItem>
              <SelectItem value="custom">自定义</SelectItem>
            </SelectPopup>
          </Select>
          {mode === 'custom' && (
            <div className="flex flex-col gap-1.5">
              <span className="text-muted-foreground text-sm">每分钟最多消息数</span>
              <NumberField
                value={value}
                onValueChange={(v) => setValue(v ?? 1)}
                min={1}
                max={600}
              >
                <NumberFieldGroup>
                  <NumberFieldDecrement />
                  <NumberFieldInput />
                  <NumberFieldIncrement />
                </NumberFieldGroup>
              </NumberField>
            </div>
          )}
        </DialogPanel>
        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>取消</DialogClose>
          <Button variant="default" onClick={save} disabled={saving}>
            保存
          </Button>
        </DialogFooter>
      </DialogPopup>
    </Dialog>
  )
}
