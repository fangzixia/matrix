import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import '@/assets/styles/global.scss'
import '@/assets/styles/pages.scss'
import '@/layouts/auth-layout.scss'
import '@/layouts/admin-layout.scss'
import '@/layouts/layouts.scss'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
