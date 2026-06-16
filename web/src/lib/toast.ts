import { toastManager } from '@/components/ui/toast'

/** 便捷的吐司封装,基于 coss-ui 的 toastManager。 */
export const toast = {
  success: (title: string, description?: string) =>
    toastManager.add({ title, description, type: 'success' }),
  error: (title: string, description?: string) =>
    toastManager.add({ title, description, type: 'error' }),
  info: (title: string, description?: string) =>
    toastManager.add({ title, description, type: 'info' }),
  warning: (title: string, description?: string) =>
    toastManager.add({ title, description, type: 'warning' }),
  message: (title: string, description?: string) =>
    toastManager.add({ title, description }),
}
