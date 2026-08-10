import React from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import './styles.css'
import Login from './pages/Login.jsx'
import Dashboard from './pages/Dashboard.jsx'
import TagDetail from './pages/TagDetail.jsx'
import Found from './pages/Found.jsx'

createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Login />} />
        <Route path="/app" element={<Dashboard />} />
        <Route path="/app/tags/:guid" element={<TagDetail />} />
        <Route path="/found/:guid" element={<Found />} />
      </Routes>
    </BrowserRouter>
  </React.StrictMode>,
)
