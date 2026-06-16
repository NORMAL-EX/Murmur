import { Routes, Route, Navigate } from 'react-router-dom'
import LoginPage from '@/pages/LoginPage'
import RegisterPage from '@/pages/RegisterPage'
import ChatPage from '@/pages/ChatPage'
import ProfilePage from '@/pages/ProfilePage'
import Dashboard from '@/pages/admin/Dashboard'
import UsersAdmin from '@/pages/admin/UsersAdmin'
import RegistrationsAdmin from '@/pages/admin/RegistrationsAdmin'
import ChannelsAdmin from '@/pages/admin/ChannelsAdmin'
import AISettingsAdmin from '@/pages/admin/AISettingsAdmin'
import SiteSettingsAdmin from '@/pages/admin/SiteSettingsAdmin'
import AuditAdmin from '@/pages/admin/AuditAdmin'
import DMAuditAdmin from '@/pages/admin/DMAuditAdmin'
import { MainLayout } from '@/components/layout/MainLayout'
import { AdminLayout } from '@/components/layout/AdminLayout'
import { RequireAuth, RequireAdmin } from '@/components/RequireAuth'

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />

      <Route
        element={
          <RequireAuth>
            <MainLayout />
          </RequireAuth>
        }
      >
        <Route path="/" element={<ChatPage />} />
        <Route path="/u/:id" element={<ProfilePage />} />
        <Route path="/me" element={<ProfilePage />} />
      </Route>

      <Route
        path="/admin"
        element={
          <RequireAdmin>
            <AdminLayout />
          </RequireAdmin>
        }
      >
        <Route index element={<Dashboard />} />
        <Route path="users" element={<UsersAdmin />} />
        <Route path="registrations" element={<RegistrationsAdmin />} />
        <Route path="channels" element={<ChannelsAdmin />} />
        <Route path="ai" element={<AISettingsAdmin />} />
        <Route path="site" element={<SiteSettingsAdmin />} />
        <Route path="dm-audit" element={<DMAuditAdmin />} />
        <Route path="audit" element={<AuditAdmin />} />
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
