import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import './styles.css'

function App() {
  return <main>Go Middle Interview Tracker</main>
}

const root = document.getElementById('root')

if (!root) {
  throw new Error('Root element was not found')
}

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
