import type { ComponentPropsWithoutRef } from 'react'
import ReactMarkdown, { defaultUrlTransform } from 'react-markdown'

export function SafeMarkdown({ children }: { children: string }) {
  return <div className="markdown"><ReactMarkdown
    skipHtml
    urlTransform={safeMarkdownURL}
    components={{ a: SafeMarkdownLink }}
  >{children}</ReactMarkdown></div>
}

function SafeMarkdownLink({ href, children, ...props }: ComponentPropsWithoutRef<'a'>) {
  if (!href) return <span>{children}</span>
  const external = href.startsWith('http://') || href.startsWith('https://')
  return <a {...props} href={href} target={external ? '_blank' : undefined} rel={external ? 'noreferrer noopener' : undefined}>{children}</a>
}

function safeMarkdownURL(url: string) {
  if (url.startsWith('#') || (url.startsWith('/') && !url.startsWith('//'))) return url
  try {
    const parsed = new URL(url)
    return parsed.protocol === 'https:' || parsed.protocol === 'http:' ? defaultUrlTransform(url) : ''
  } catch {
    return ''
  }
}
