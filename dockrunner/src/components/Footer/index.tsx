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
        </div>

        <div className={styles.footer__version}>{version}</div>

      </div>
    </footer>
  )
}
