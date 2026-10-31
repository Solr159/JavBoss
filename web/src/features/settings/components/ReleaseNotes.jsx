import { useMemo } from 'react'
import createDOMPurify from 'dompurify'
import '@/features/settings/components/ReleaseNotes.css'

const purifier = createDOMPurify(window)
purifier.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A' && node.hasAttribute('href')) {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer')
  }
})

const policy = {
  ALLOWED_TAGS: [
    'h1',
    'h2',
    'h3',
    'h4',
    'h5',
    'h6',
    'p',
    'br',
    'hr',
    'div',
    'span',
    'ul',
    'ol',
    'li',
    'strong',
    'b',
    'em',
    'i',
    'del',
    's',
    'a',
    'code',
    'pre',
    'blockquote',
    'table',
    'thead',
    'tbody',
    'tr',
    'th',
    'td',
    'img',
    'kbd',
    'sup',
    'sub',
  ],
  ALLOWED_ATTR: ['href', 'title', 'src', 'alt', 'colspan', 'rowspan', 'start'],
  ALLOW_DATA_ATTR: false,
  ALLOW_ARIA_ATTR: false,
  ALLOWED_URI_REGEXP: /^(?:https?:|mailto:)/i,
}

export default function ReleaseNotes({ html, text }) {
  const markup = useMemo(() => (html ? purifier.sanitize(html, policy) : ''), [html])
  return markup ? (
    <div
      className="release-notes text-sm text-zinc-600"
      dangerouslySetInnerHTML={{ __html: markup }}
    />
  ) : (
    <p className="whitespace-pre-wrap break-words text-sm text-zinc-600">{text}</p>
  )
}
