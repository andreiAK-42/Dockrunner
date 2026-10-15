// Мини-рендер markdown для документации.
// Форматирование (**bold**, *italic*, `code`) применяется только к обычному
// тексту: код, картинки и ссылки прячутся в плейсхолдеры до форматирования,
// иначе `_` внутри URL (`volume_4.jpg`) съедались emphasis-правилами.

export function escapeHtml(source: string) {
  const withAmp = source.replace(/&/g, '&amp;')
  const withLt = withAmp.replace(/</g, '&lt;')
  return withLt.replace(/>/g, '&gt;')
}

export function normalizeImgSrc(rawSrc: string): string {
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

export function renderImage(match: string, alt: string, src: string, placeholderClass = ''): string {
  const normalizedSrc = normalizeImgSrc(src)
  const safeAlt = escapeHtml(alt || 'скриншот')
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

export function mdToHtml(md: string, placeholderClass = ''): string {
  // Файлы правятся на Windows (CRLF): без нормализации сплиттер `\n{2,}`
  // не находит границы абзацев и весь док склеивается в один блок.
  const normalized = md.replace(/\r\n?/g, '\n')
  const kept: string[] = []
  const stash = (html: string) => `\u0000${kept.push(html) - 1}\u0000`
  const restore = (html: string) => html.replace(/\u0000(\d+)\u0000/g, (_m, i) => kept[Number(i)] ?? '')

  const withCodeBlocks = normalized.replace(/^```(\w*)\n([\s\S]*?)```/gm, (_match, _lang, code) => {
    return stash(`<pre><code>${escapeHtml(code.trim())}</code></pre>`)
  })

  const withH3 = withCodeBlocks.replace(/^### (.*)$/gm, '<h3>$1</h3>')
  const withH2 = withH3.replace(/^## (.*)$/gm, '<h2>$1</h2>')
  const withH1 = withH2.replace(/^# (.*)$/gm, '<h1>$1</h1>')
  const withQuotes = withH1.replace(/^> (.*)$/gm, '<blockquote>$1</blockquote>')
  const withImages = withQuotes.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, (match, alt, src) => stash(renderImage(match, alt, src, placeholderClass)))
  const withLinks = withImages.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_match, text, url) => stash(`<a href="${url}" target="_blank" rel="noreferrer">${text}</a>`))
  const withBoldStars = withLinks.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  const withBoldUnderscores = withBoldStars.replace(/__([^_]+)__/g, '<strong>$1</strong>')
  const withItalicStars = withBoldUnderscores.replace(/\*([^*]+)\*/g, '<em>$1</em>')
  const withItalicUnderscores = withItalicStars.replace(/_([^_]+)_/g, '<em>$1</em>')
  const withCode = withItalicUnderscores.replace(/`([^`]+)`/g, (_match, code) => stash(`<code>${escapeHtml(code)}</code>`))
  const withBullets = withCode.replace(/^- (.*)$/gm, '<li>$1</li>')
  const withOrderedItems = withBullets.replace(/^\d+\. (.*)$/gm, '<li>$1</li>')

  const withLists = withOrderedItems.replace(/(<li>.*<\/li>\n?)+/g, (match) => `<ul>${match}</ul>`)
  const blocks = withLists.split(/\n{2,}/)
  const wrappedBlocks = blocks.map(wrapBlock)

  return restore(wrappedBlocks.join('\n'))
}
