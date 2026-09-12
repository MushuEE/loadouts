import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import { SessionProvider } from './session/SessionContext.tsx'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <SessionProvider>
      <App />
    </SessionProvider>
  </React.StrictMode>,
)
