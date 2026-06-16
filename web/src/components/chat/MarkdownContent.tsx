import { useEffect, useState, type ReactNode } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { useNavigate } from 'react-router-dom'
import { useChat } from '@/contexts/ChatContext'
import type { User } from '@/lib/types'

/** Full-screen image preview. Click anywhere or press Esc to close. */
function Lightbox({ src, onClose }: { src: string; onClose: () => void }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prev
    }
  }, [onClose])
  return (
    <div
      onClick={onClose}
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/80 p-4"
      role="dialog"
      aria-modal="true"
    >
      <img src={src} alt="" className="max-h-full max-w-full rounded-lg object-contain" />
    </div>
  )
}

const mentionRe = /(@[A-Za-z0-9_]{1,32})/g

/** Replace @username tokens in a plain string with clickable, highlighted chips. */
function highlightMentions(
  text: string,
  resolve: (name: string) => User | undefined,
  onClick: (u: User) => void,
  keyPrefix: string,
): ReactNode[] {
  const parts = text.split(mentionRe)
  return parts.map((part, i) => {
    if (i % 2 === 1) {
      const u = resolve(part.slice(1).toLowerCase())
      if (u) {
        return (
          <button
            key={`${keyPrefix}-${i}`}
            type="button"
            onClick={() => onClick(u)}
            className="rounded bg-primary/10 px-1 font-medium text-primary hover:bg-primary/20"
          >
            @{u.nickname || u.username}
          </button>
        )
      }
    }
    return part
  })
}

export function MarkdownContent({ content }: { content: string }) {
  const { members } = useChat()
  const navigate = useNavigate()
  const [zoom, setZoom] = useState<string | null>(null)

  const byName = new Map(members.map((m) => [m.username.toLowerCase(), m]))
  const resolve = (name: string) => byName.get(name)
  const onClick = (u: User) => navigate(`/u/${u.id}`)

  // Walk react-markdown children and highlight mentions inside string nodes.
  const proc = (children: ReactNode, key: string): ReactNode => {
    if (typeof children === 'string') {
      return highlightMentions(children, resolve, onClick, key)
    }
    if (Array.isArray(children)) {
      return children.map((c, i) =>
        typeof c === 'string' ? (
          <span key={`${key}-${i}`}>{highlightMentions(c, resolve, onClick, `${key}-${i}`)}</span>
        ) : (
          c
        ),
      )
    }
    return children
  }

  return (
    <>
    <div className="break-words text-sm leading-relaxed [word-break:break-word]">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          p: ({ children }) => <p className="my-0.5 whitespace-pre-wrap">{proc(children, 'p')}</p>,
          li: ({ children }) => <li className="ml-4 list-disc">{proc(children, 'li')}</li>,
          a: ({ href, children }) => (
            <a
              href={href}
              target="_blank"
              rel="noreferrer noopener"
              className="text-primary underline underline-offset-2"
            >
              {children}
            </a>
          ),
          img: ({ src, alt }) =>
            typeof src === 'string' ? (
              <img
                src={src}
                alt={alt || '图片'}
                loading="lazy"
                onClick={() => setZoom(src)}
                className="my-1 max-h-80 max-w-full cursor-zoom-in rounded-lg border border-border object-contain"
              />
            ) : null,
          code: ({ children }) => (
            <code className="rounded bg-muted px-1 py-0.5 font-mono text-[0.85em]">{children}</code>
          ),
          pre: ({ children }) => (
            <pre className="my-1 overflow-x-auto rounded-lg bg-muted p-3 font-mono text-xs">
              {children}
            </pre>
          ),
          blockquote: ({ children }) => (
            <blockquote className="border-border border-l-2 pl-3 text-muted-foreground">
              {children}
            </blockquote>
          ),
          strong: ({ children }) => <strong className="font-semibold">{proc(children, 'st')}</strong>,
          em: ({ children }) => <em>{proc(children, 'em')}</em>,
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
      {zoom && <Lightbox src={zoom} onClose={() => setZoom(null)} />}
    </>
  )
}

export default MarkdownContent
