import { useEffect, useState } from 'react'
import { Box, Button, Typography } from '@mui/material'
import { Server, ArrowLeft, BookOpen, Image as ImageIcon } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import Footer from '../../components/Footer'
import styles from './Docs.module.scss'

const DOCS = [
  { id: 'intro', title: 'Введение', file: '/docs/intro.md' },
  { id: 'quickstart', title: 'Быстрый старт', file: '/docs/quickstart.md' },
  { id: 'deploy', title: 'Деплой и обновления', file: '/docs/deploy.md' },
]

function escapeHtml(source: string) {
  const withAmp = source.replace(/&/g, '&amp;')
  const withLt = withAmp.replace(/</g, '&lt;')
  return withLt.replace(/>/g, '&gt;')
}

function normalizeImgSrc(rawSrc: string): string {
  const src = rawSrc.trim()

  if (src.startsWith('http') || src.startsWith('/')) {
    return src
  }

  const cleaned = src.replace(/^(\.\/|\.\.\/)+/, '')

  const screenshotsIndex = cleaned.indexOf('screenshots/')
  if (screenshotsIndex >= 0) {
    return '/docs/' + cleaned.substring(screenshotsIndex)
  }

  if (cleaned.startsWith('docs/')) {
    return '/' + cleaned
  }

  return '/docs/' + cleaned
}

function renderImage(match: string, alt: string, src: string): string {
  const normalizedSrc = normalizeImgSrc(src)
  const safeAlt = escapeHtml(alt || 'скриншот')
  const placeholderClass = styles.placeholderImg
  return `<img src="${normalizedSrc}" alt="${safeAlt}" loading="lazy" style="max-width:100%;border-radius:10px;border:1px solid var(--custom-divider);display:block;margin:12px 0" onerror="this.style.display='none';var p=this.nextElementSibling;if(p) p.style.display='flex';" /><div class="${placeholderClass}" style="display:none"><span style="display:flex;align-items:center;gap:6px"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21"/></svg> Скриншот: ${safeAlt}</span><span style="font-size:0.7rem;opacity:.7">файл не найден: ${normalizedSrc} — положите PNG в public/docs/screenshots/</span></div>`
}

function isRawHtmlBlock(text: string) {
  return (
    text.startsWith('<h') ||
    text.startsWith('<pre') ||
    text.startsWith('<ul') ||
    text.startsWith('<blockquote') ||
    text.startsWith('<div') ||
    text.startsWith('<img')
  )
}

function wrapBlock(block: string) {
  const text = block.trim()
  if (!text) {
    return ''
  }
  if (isRawHtmlBlock(text)) {
    return text
  }
  const withBreaks = text.replace(/\n/g, '<br/>')
  return `<p>${withBreaks}</p>`
}

function mdToHtml(md: string): string {
  const withCodeBlocks = md.replace(/^```(\w*)\n([\s\S]*?)```/gm, (_match, _lang, code) => {
    return `<pre><code>${escapeHtml(code.trim())}</code></pre>`
  })

  const withH3 = withCodeBlocks.replace(/^### (.*)$/gm, '<h3>$1</h3>')
  const withH2 = withH3.replace(/^## (.*)$/gm, '<h2>$1</h2>')
  const withH1 = withH2.replace(/^# (.*)$/gm, '<h1>$1</h1>')
  const withQuotes = withH1.replace(/^> (.*)$/gm, '<blockquote>$1</blockquote>')
  const withImages = withQuotes.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, renderImage)
  const withLinks = withImages.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer">$1</a>')
  const withBoldStars = withLinks.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  const withBoldUnderscores = withBoldStars.replace(/__([^_]+)__/g, '<strong>$1</strong>')
  const withItalicStars = withBoldUnderscores.replace(/\*([^*]+)\*/g, '<em>$1</em>')
  const withItalicUnderscores = withItalicStars.replace(/_([^_]+)_/g, '<em>$1</em>')
  const withCode = withItalicUnderscores.replace(/`([^`]+)`/g, '<code>$1</code>')
  const withBullets = withCode.replace(/^- (.*)$/gm, '<li>$1</li>')
  const withOrderedItems = withBullets.replace(/^\d+\. (.*)$/gm, '<li>$1</li>')

  const withLists = withOrderedItems.replace(/(<li>.*<\/li>\n?)+/g, (match) => `<ul>${match}</ul>`)
  const blocks = withLists.split(/\n{2,}/)
  const wrappedBlocks = blocks.map(wrapBlock)

  return wrappedBlocks.join('\n')
}

export default function Docs() {
  const navigate = useNavigate()
  const [active, setActive] = useState(DOCS[0].id)
  const [content, setContent] = useState<string>('Загрузка...')
  const [loading, setLoading] = useState(true)

  const activeDoc = DOCS.find((doc) => doc.id === active) || DOCS[0]

  useEffect(() => {
    setLoading(true)
    fetch(activeDoc.file)
      .then((response) => {
        if (response.ok) {
          return response.text()
        }
        return Promise.reject(response.statusText)
      })
      .then((markdown) => setContent(mdToHtml(markdown)))
      .catch(() => setContent(`<p>Не удалось загрузить <code>${activeDoc.file}</code>. Убедитесь что файл лежит в <code>public/docs/</code>.</p>`))
      .finally(() => setLoading(false))
  }, [activeDoc.file])

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div className={styles.header__inner}>
          <div className={styles.header__brand} onClick={() => navigate('/')}>
            <div className={styles.header__logo}><Server size={16} /></div>
            <span style={{ fontWeight: 800 }}>Dockrunner</span>
            <span style={{ marginLeft: 8, fontSize: '.7rem', opacity: .6, display: 'flex', alignItems: 'center', gap: 4 }}><BookOpen size={12} /> Docs</span>
          </div>
          <Button size="small" startIcon={<ArrowLeft size={14} />} onClick={() => navigate(-1)}>Назад</Button>
        </div>
      </header>

      <div className={styles.layout}>
        <aside className={styles.sidebar}>
          <div className={styles.sidebar__title}>Разделы</div>
          <div className={styles.sidebar__list}>
            {DOCS.map((doc) => (
              <div key={doc.id} onClick={() => setActive(doc.id)} className={`${styles.sidebar__item} ${active === doc.id ? styles['sidebar__item--active'] : ''}`}>{doc.title}</div>
            ))}
          </div>
          <Box sx={{ p: 1.5, borderTop: '1px solid', borderColor: 'divider', mt: 1 }}>
            <Typography variant="caption" color="text.secondary" sx={{ display: 'flex', gap: 0.5, alignItems: 'center' }}><ImageIcon size={12} /> Скриншоты — заглушки, замените на реальные PNG в <code>public/docs/screenshots/</code></Typography>
          </Box>
        </aside>

        <main className={styles.content}>
          <div className={styles.article}>
            {loading ? <div className={styles.loading}>Загрузка {activeDoc.file}...</div> : (
              <div dangerouslySetInnerHTML={{ __html: content }} />
            )}
            <Box sx={{ mt: 3, p: 1.5, borderRadius: 2, bgcolor: 'background.default', border: '1px solid', borderColor: 'divider' }}>
              <Typography variant="caption" color="text.secondary">Документация подтягивается из <code>public/docs/*.md</code> через <code>fetch()</code>. Добавьте новые <code>.md</code> файлы и пропишите их в массиве <code>DOCS</code> в <code>src/pages/Docs/index.tsx</code>.</Typography>
            </Box>
          </div>
        </main>
      </div>

      <Footer />
    </div>
  )
}
