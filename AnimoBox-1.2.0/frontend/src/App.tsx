import React from 'react';
import { HashRouter, Routes, Route } from 'react-router-dom';
import Navbar from './components/Navbar';
import Home from './pages/Home';
import Search from './pages/Search';
import AnimeDetail from './pages/AnimeDetail';
import Watch from './pages/Watch';
import Library from './pages/Library';
import Settings from './pages/Settings';
import History from './pages/History';

export default function App() {
  return (
    <HashRouter>
      <div style={{ display: 'flex', width: '100vw', height: '100vh', background: 'var(--bg)' }}>
        <Navbar />
        <main style={{
          marginLeft: 64,
          flex: 1,
          height: '100vh',
          overflow: 'auto',
          background: 'var(--bg)',
        }}>
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/search" element={<Search />} />
            <Route path="/anime/:id" element={<AnimeDetail />} />
            <Route path="/watch/:episodeId" element={<Watch />} />
            <Route path="/library" element={<Library />} />
            <Route path="/history" element={<History />} />
            <Route path="/settings" element={<Settings />} />
          </Routes>
        </main>
      </div>
    </HashRouter>
  );
}
