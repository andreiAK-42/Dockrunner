import { useEffect, useState } from 'react'
import { Button } from '@mui/material'
import { Server, ArrowLeft } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import Footer from '../../components/Footer'
import { mdToHtml } from './markdown'
import styles from './Docs.module.scss'

const DOCS = [
  { id: 'intro', title: 'Введение', file: '/docs/intro.md' },
    { id: 'base', title: 'Контейнеры, порты, тома и env', file: '/docs/base.md' },
  { id: 'quickstart', title: 'Быстрый старт', file: '/docs/quickstart.md' },
]

function renderDoc(markdown: string): string {
  return mdToHtml(markdown, styles.placeholderImg)
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
      .then((markdown) => setContent(renderDoc(markdown)))
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
        </aside>

        <main className={styles.content}>
          <div className={styles.article}>
            {loading ? <div className={styles.loading}>Загрузка {activeDoc.file}...</div> : (
              <div dangerouslySetInnerHTML={{ __html: content }} />
            )}
          </div>
        </main>
      </div>

      <Footer />
    </div>
  )
}
