import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route, NavLink } from 'react-router-dom'
import { Dashboard } from './pages/Dashboard'
import { HITLWorkbench } from './pages/HITLWorkbench'
import { Analytics } from './pages/Analytics'

const NAV_STYLE: React.CSSProperties = {
  display: 'flex', gap: 24, padding: '16px 24px',
  borderBottom: '1px solid #262626', background: '#0a0a0a',
}
const LINK_STYLE: React.CSSProperties = { color: '#a3a3a3', textDecoration: 'none', fontSize: 14, fontWeight: 500 }
const ACTIVE_STYLE: React.CSSProperties = { ...LINK_STYLE, color: '#fff' }

function App() {
  return (
    <BrowserRouter>
      <nav style={NAV_STYLE}>
        <span style={{ color: '#fff', fontWeight: 700, marginRight: 16 }}>Nexoraa</span>
        <NavLink to="/" style={({ isActive }) => isActive ? ACTIVE_STYLE : LINK_STYLE}>Dashboard</NavLink>
        <NavLink to="/hitl" style={({ isActive }) => isActive ? ACTIVE_STYLE : LINK_STYLE}>HITL Workbench</NavLink>
        <NavLink to="/analytics" style={({ isActive }) => isActive ? ACTIVE_STYLE : LINK_STYLE}>Analytics</NavLink>
      </nav>
      <main style={{ padding: 24 }}>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/hitl" element={<HITLWorkbench />} />
          <Route path="/analytics" element={<Analytics />} />
        </Routes>
      </main>
    </BrowserRouter>
  )
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
