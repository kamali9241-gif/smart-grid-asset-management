import { NavLink, Route, Routes, Navigate } from 'react-router-dom'
import { ImportPage } from './components/ImportPage'
import { ExplorerPage } from './components/ExplorerPage'

export default function App() {
  return (
    <div className="app-shell">
      <header className="app-header">
        <h1 className="app-title">Smart Grid Asset Explorer</h1>
        <nav className="app-nav">
          <NavLink to="/import" className={({ isActive }) => (isActive ? 'nav-active' : '')}>
            Import
          </NavLink>
          <NavLink to="/explorer" className={({ isActive }) => (isActive ? 'nav-active' : '')}>
            Explorer
          </NavLink>
        </nav>
      </header>
      <div className="app-body">
        <Routes>
          <Route path="/" element={<Navigate to="/import" replace />} />
          <Route path="/import" element={<ImportPage />} />
          <Route path="/explorer" element={<ExplorerPage />} />
          <Route path="/explorer/:assetId" element={<ExplorerPage />} />
          <Route path="*" element={<Navigate to="/import" replace />} />
        </Routes>
      </div>
    </div>
  )
}
