import { useState, useEffect } from 'react'
import { Box, Typography, Tabs, Tab, Card, CardContent, Button, Divider } from '@mui/material'
import { Server, ShieldCheck, Lock, ArrowLeft } from 'lucide-react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import Footer from '../../components/Footer'
import styles from './Privacy.module.scss'

const PRIVACY_TEXT = `
Мы в Dockyard верим, что доверие — это основа любых отношений. Поэтому собираем минимум данных, честно рассказываем зачем они нужны и никогда не передаём их третьим лицам без вашего согласия.

Что мы собираем: адрес электронной почты для входа, технические логи для стабильной работы кластера и анонимную аналитику использования интерфейса. Всё это помогает нам делать продукт быстрее и удобнее, а не следить за вами.

Как храним: данные шифруются при передаче и хранении, доступ к ним есть только у ограниченной команды с двухфакторной аутентификацией. Мы регулярно проверяем настройки безопасности и обновляем инфраструктуру.

Ваши права: вы всегда можете запросить копию своих данных, попросить их исправить или удалить. Просто напишите на privacy@dockyard.local — отвечаем мягко, по-человечески и обычно в течение пары дней.

Мы не любим спам и не будем присылать рассылки без явного согласия. Если что-то изменится в этой политике, мы предупредим заранее и понятным языком. Спасибо, что доверяете нам свои проекты!
`.trim()

const PROCESSING_TEXT = `
Обработка персональных данных в Dockyard устроена просто и прозрачно. Мы обрабатываем только то, что действительно нужно для работы сервиса, и ровно столько, сколько нужно.

Цель обработки: создание аккаунта, аутентификация, биллинг (если подключите), поддержка и улучшение качества. Правовое основание — ваше согласие при регистрации и исполнение договора об использовании платформы.

  Объём: имя (как вы представились), логин, IP-адрес и технические идентификаторы сессии. Никаких лишних полей, никаких скрытых трекеров.

Срок: пока существует аккаунт. После удаления мы стираем данные в течение 30 дней, кроме логов безопасности, которые хранятся до 6 месяцев для расследования инцидентов — и то в обезличенном виде.

Передача: мы не продаём данные. Подрядчики (хостинг, почта) получают доступ только по договору обработки и с теми же обязательствами по защите. Трансграничная передача — только в страны с адекватным уровнем защиты или с вашего согласия.

Вы можете отозвать согласие, запросить доступ, исправление, удаление или перенос данных. Для этого достаточно письма на dpo@dockyard.local. Мы отвечаем спокойно, без формальностей, и поможем на каждом шаге. Ваша приватность для нас — не формальность, а уважение.
`.trim()

export default function Privacy() {
  const [searchParams, setSearchParams] = useSearchParams()
  const initial = searchParams.get('tab') === 'processing' ? 1 : 0
  const [tab, setTab] = useState(initial)
  const navigate = useNavigate()
  useEffect(() => { setSearchParams({ tab: tab === 0 ? 'privacy' : 'processing' }, { replace: true }) }, [tab, setSearchParams])

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div className={styles.header__inner}>
          <div className={styles.header__brand} onClick={() => navigate('/')}>
            <div className={styles.header__logo}><Server size={16} /></div>
            <span style={{ fontWeight: 800 }}>Dockyard</span>
          </div>
          <Button size="small" startIcon={<ArrowLeft size={14} />} onClick={() => navigate(-1)}>Назад</Button>
        </div>
      </header>

      <div className={styles.content}>
        <h1 className={styles.content__title}>Правовые документы</h1>

        <Card className={styles.card}>
          <Tabs value={tab} onChange={(_, v) => setTab(v)} className={styles.tabs}>
            <Tab icon={<ShieldCheck size={16} />} iconPosition="start" label="Политика конфиденциальности" sx={{ textTransform: 'none', fontWeight: 600 }} />
            <Tab icon={<Lock size={16} />} iconPosition="start" label="Обработка персональных данных" sx={{ textTransform: 'none', fontWeight: 600 }} />
          </Tabs>
          <CardContent className={styles.body}>
            {tab === 0 ? (
              <>
                <div className={styles.body__title}>Политика конфиденциальности</div>
                <div className={styles.body__meta}>Обновлено: {new Date().toLocaleDateString('ru-RU')}</div>
                <Divider sx={{ my: 1.5 }} />
                <div className={styles.body__text}>{PRIVACY_TEXT}</div>
              </>
            ) : (
              <>
                <div className={styles.body__title}>Политика обработки персональных данных</div>
                <div className={styles.body__meta}>Обновлено: {new Date().toLocaleDateString('ru-RU')}</div>
                <Divider sx={{ my: 1.5 }} />
                <div className={styles.body__text}>{PROCESSING_TEXT}</div>
              </>
            )}
          </CardContent>
        </Card>

        <Box sx={{ mt: 2, display: 'flex', justifyContent: 'center' }}><Button variant="text" size="small" onClick={() => navigate('/')}>На главную</Button></Box>
      </div>

      <Footer />
    </div>
  )
}
