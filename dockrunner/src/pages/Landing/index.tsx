import { useEffect, useState, useCallback } from 'react'
import { Box, Button, Card, CardContent, Typography, Container, Stack, IconButton } from '@mui/material'
import { Server, Rocket, ShieldCheck, Globe, Cpu, Database, ArrowRight, ChevronLeft, ChevronRight, Layers, Activity } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../../auth/AuthContext'
import Footer from '../../components/Footer'
import styles from './Landing.module.scss'

const ADVANTAGES = [
  { icon: <Rocket size={28} />, title: 'Деплой за минуты', desc: 'Вставьте ссылку своего проекта на Git-систему и получите рабочий сервис с доменом и HTTPS за считанные минуты. Никаких пайплайнов вручную.', color: '#3b82f6' },
  { icon: <Server size={28} />, title: 'Разные сервера', desc: 'Различный выбор серверов по CPU, RAM и диску. Балансировка и отказоустойчивость из коробки - без ручного размещения.', color: '#10b981' },
  { icon: <Globe size={28} />, title: 'Домены и TLS', desc: 'Caddy сам выпускает сертификаты, настраивает маршрутизацию и обновляет их. Просто купите домен и укажите его в нашем сервисе.', color: '#8b5cf6' },
  { icon: <Database size={28} />, title: 'Уже готовые интеграции с популярными базами данных', desc: 'Postgre, Mongo, Redis уже готовы, а если нужно базы данных нет, то можете развернуть её сами!', color: '#f59e0b' },
  { icon: <Activity size={28} />, title: 'Наблюдаемость', desc: 'Метрики CPU/RAM/Disk по нодам и приложениям, карта серверов и лента деплоев в реальном времени.', color: '#ef4444' },
]

function Header() {
  const { isAuthenticated } = useAuth()
  const navigate = useNavigate()
  return (
    <header className={styles.header}>
      <div className={styles.header__inner}>
        <div className={styles.header__brand} onClick={() => navigate('/')}>
          <div className={styles.header__logo}><Server size={20} /></div>
          <span className={styles.header__title}>Dockrunner</span>
        </div>
        <div className={styles.header__actions}>
          <Button variant="text" size="small" onClick={() => navigate('/docs')} sx={{ display: { xs: 'none', sm: 'inline-flex' } }}>Документация</Button>
          {isAuthenticated ? (
            <Button variant="contained" endIcon={<ArrowRight size={16} />} onClick={() => navigate('/dashboard')}>Личный кабинет</Button>
          ) : (
            <Button variant="contained" onClick={() => navigate('/login')}>Войти</Button>
          )}
        </div>
      </div>
    </header>
  )
}

function AdvantageCarousel() {
  const [index, setIndex] = useState(0)
  const [paused, setPaused] = useState(false)
  const next = useCallback(() => setIndex((i) => (i + 1) % ADVANTAGES.length), [])
  const prev = useCallback(() => setIndex((i) => (i - 1 + ADVANTAGES.length) % ADVANTAGES.length), [])
  useEffect(() => {
    if (paused) return
    const id = setInterval(next, 5000)
    return () => clearInterval(id)
  }, [next, paused])

  return (
    <div className={styles.carousel} onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)}>
      <div className={styles.carousel__viewport}>
        <div className={styles.carousel__track} style={{ transform: `translateX(-${index * 100}%)` }}>
          {ADVANTAGES.map((a) => (
            <div key={a.title} className={styles.carousel__slide}>
              <div className={styles.carousel__icon} style={{ background: `${a.color}18`, color: a.color }}>{a.icon}</div>
              <div className={styles.carousel__content}>
                <div className={styles.carousel__title}>{a.title}</div>
                <div className={styles.carousel__desc}>{a.desc}</div>
              </div>
              <div className={styles.carousel__counter}>{index + 1} / {ADVANTAGES.length}</div>
            </div>
          ))}
        </div>
      </div>
      <div className={styles.carousel__dots}>
        {ADVANTAGES.map((_, i) => (
          <div key={i} onClick={() => setIndex(i)} className={`${styles.carousel__dot} ${i === index ? styles['carousel__dot--active'] : ''}`} />
        ))}
      </div>
      <IconButton onClick={prev} className={`${styles.carousel__arrow} ${styles['carousel__arrow--prev']}`} size="small"><ChevronLeft size={16} /></IconButton>
      <IconButton onClick={next} className={`${styles.carousel__arrow} ${styles['carousel__arrow--next']}`} size="small"><ChevronRight size={16} /></IconButton>
    </div>
  )
}

export default function Landing() {
  const navigate = useNavigate()
  const { isAuthenticated } = useAuth()
  return (
    <div className={styles.page}>
      <Header />
      <Container maxWidth="lg" sx={{ pt: { xs: 4, md: 6 }, pb: 2 }}>
        <div className={styles.hero}>
          <div className={styles.hero__overline}>Просто. Быстро. Надёжно.</div>
          <h2 className={styles.hero__title}>Сервис для запуска <span>ИТ-проектов</span>  без боли</h2>
          <p className={styles.hero__subtitle}>Dockrunner - это CI/CD сервис, для тестирования и развёртки студенческих ИТ-проектов. Укажите ссылку на ваш репозиторий и сервис развернёт ваше приложение.</p>
          <div className={styles.hero__cta}>
            <Button size="large" variant="contained" endIcon={<ArrowRight size={18} />} onClick={() => navigate(isAuthenticated ? '/dashboard' : '/login')}>
              {isAuthenticated ? 'Перейти в кабинет' : 'Начать бесплатно'}
            </Button>
            <Button size="large" variant="outlined" onClick={() => document.getElementById('advantages')?.scrollIntoView({ behavior: 'smooth' })}>Посмотреть возможности</Button>
          </div>
        </div>

        <div id="advantages"><AdvantageCarousel /></div>

        <div className={styles.infoGrid}>
          {[
            { icon: <ShieldCheck size={20} />, title: 'Безопасность', text: 'Изоляция по серверам, раздельные хранилища и различные методы безопасности.' },
            { icon: <Cpu size={20} />, title: 'Экономия ресурсов', text: 'Планировщик учитывает реальную загрузку CPU/RAM/Disk и подбирает сервер с запасом.' },
            { icon: <Layers size={20} />, title: 'Подходит всем', text: 'Достаточно только иметь в своём проекте рабочий Dockerfile или docker compose, остальное сделает наша система.' },
          ].map((b) => (
            <Card key={b.title} variant="outlined">
              <CardContent sx={{ p: 2.5 }}>
                <div className={styles.infoGrid__icon}>{b.icon}</div>
                <Typography variant="subtitle1" sx={{ fontWeight: 700, mb: 0.5 }}>{b.title}</Typography>
                <Typography variant="body2" color="text.secondary" sx={{ lineHeight: 1.6 }}>{b.text}</Typography>
              </CardContent>
            </Card>
          ))}
        </div>


      </Container>
      <Box sx={{ flexGrow: 1 }} />
      <Footer />
    </div>
  )
}
