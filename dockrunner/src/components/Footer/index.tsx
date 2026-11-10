import { Typography } from '@mui/material'
import { Server } from 'lucide-react'
import { APP_NAME, APP_VERSION } from '../../config/appConfig'
import styles from './Footer.module.scss'


export default function Footer() {
  return (
    <footer className={styles.footer}>
      <div className={styles.footer__inner}>
        <div className={styles.footer__brand}>
          <div className={styles.footer__logo}><Server size={16} /></div>
          <Typography variant="body2" sx={{ fontWeight: 700 }}>{APP_NAME}</Typography>
        </div>

        <div className={styles.footer__version}>{APP_VERSION}</div>

      </div>
    </footer>
  )
}
