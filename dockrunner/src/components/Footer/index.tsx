import { Typography, Button } from '@mui/material'
import { Server } from 'lucide-react'
import { Link } from 'react-router-dom'
import { APP_NAME, APP_VERSION, APP_COPYRIGHT } from '../../config/appConfig'
import styles from './Footer.module.scss'

interface FooterProps {
  version?: string
  hideLinks?: boolean
}

export default function Footer({ version = APP_VERSION, hideLinks = false }: FooterProps) {
  return (
    <footer className={styles.footer}>
      <div className={styles.footer__inner}>
        <div className={styles.footer__brand}>
          <div className={styles.footer__logo}><Server size={16} /></div>
          <Typography variant="body2" sx={{ fontWeight: 700 }}>{APP_NAME}</Typography>
          <span className={styles.footer__copy}>© {new Date().getFullYear()} {APP_COPYRIGHT}</span>
        </div>

        <div className={styles.footer__version}>{version}</div>

        {!hideLinks && (
          <div className={styles.footer__links}>
            <Button component={Link} to="/privacy?tab=privacy" variant="text" size="small" sx={{ color: 'text.secondary', textTransform: 'none', fontSize: '0.75rem' }}>
              Политика конфиденциальности
            </Button>
            <div className={styles.footer__dot} />
            <Button component={Link} to="/privacy?tab=processing" variant="text" size="small" sx={{ color: 'text.secondary', textTransform: 'none', fontSize: '0.75rem' }}>
              Обработка персональных данных
            </Button>
          </div>
        )}
      </div>
    </footer>
  )
}
