import { Sun, Moon, Check } from 'lucide-react'
import { CircleHalf } from '@/components/icons'
import { Button } from '@/components/ui/button'
import { useTheme } from '@/contexts/ThemeContext'
import {
  Menu as DropdownMenu,
  MenuPopup,
  MenuItem,
  MenuTrigger,
} from '@/components/ui/menu'

/**
 * Light / dark / system theme switcher. Mirrors the Cloud-PE Header pattern:
 * an outline `button-header` icon trigger that opens an animated coss-ui Menu
 * with the three options and a Check on the active one.
 */
export function ThemeToggle({ className }: { className?: string }) {
  const { theme, setTheme, resolvedTheme } = useTheme()

  const icon =
    theme === 'system' ? (
      <CircleHalf className="h-[18px] w-[18px]" />
    ) : resolvedTheme === 'dark' ? (
      <Moon className="h-[18px] w-[18px]" />
    ) : (
      <Sun className="h-[18px] w-[18px]" />
    )

  return (
    <DropdownMenu>
      <MenuTrigger
        render={
          <Button
            variant="outline"
            size="icon"
            className={`rounded-lg button-header ${className ?? ''}`}
            aria-label="切换主题"
          >
            {icon}
            <span className="sr-only">切换主题</span>
          </Button>
        }
      />
      <MenuPopup className="min-w-[140px] menu-popup-animated">
        <MenuItem onClick={() => setTheme('light')} className="flex items-center gap-2">
          <Sun className="h-4 w-4" />
          浅色模式
          {theme === 'light' && <Check className="ml-auto h-4 w-4" />}
        </MenuItem>
        <MenuItem onClick={() => setTheme('dark')} className="flex items-center gap-2">
          <Moon className="h-4 w-4" />
          深色模式
          {theme === 'dark' && <Check className="ml-auto h-4 w-4" />}
        </MenuItem>
        <MenuItem onClick={() => setTheme('system')} className="flex items-center gap-2">
          <CircleHalf className="h-4 w-4" />
          跟随系统
          {theme === 'system' && <Check className="ml-auto h-4 w-4" />}
        </MenuItem>
      </MenuPopup>
    </DropdownMenu>
  )
}

export default ThemeToggle
