import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { IconLibrary, IconTrash, IconStar, IconDownload, IconUpload } from '@tabler/icons-react';
import { LibraryAnime } from '../types';
import { GetLibrary, UpdateLibraryItem, RemoveFromLibrary, SaveLibraryToFile, ImportLibraryFromFile } from '../../wailsjs/go/main/App';

const STATUS_OPTIONS = [
  { value: 'all', label: 'All' },
  { value: 'watching', label: 'Watching' },
  { value: 'completed', label: 'Completed' },
  { value: 'plantowatch', label: 'Plan to Watch' },
  { value: 'dropped', label: 'Dropped' },
  { value: 'onhold', label: 'On Hold' },
];

const STATUS_COLORS: Record<string, string> = {
  watching: '#339af0',
  completed: '#51cf66',
  plantowatch: '#ffc107',
  dropped: '#ff6b6b',
  onhold: '#ff922b',
};

export default function Library() {
  const navigate = useNavigate();
  const [library, setLibrary] = useState<LibraryAnime[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('all');

  useEffect(() => { loadLibrary(); }, []);

  const loadLibrary = async () => {
    setLoading(true);
    try {
      const data = await GetLibrary();
      setLibrary((data as any) || []);
    } catch { setLibrary([]); } finally { setLoading(false); }
  };

  const handleStatusChange = async (animeId: string, newStatus: string) => {
    try {
      const item = library.find((l) => l.animeId === animeId);
      if (item) {
        await UpdateLibraryItem(animeId, newStatus, item.score, item.episodesWatch);
        setLibrary((prev) => prev.map((l) => l.animeId === animeId ? { ...l, status: newStatus } : l));
      }
    } catch {}
  };

  const handleRemove = async (animeId: string) => {
    try { await RemoveFromLibrary(animeId); setLibrary((prev) => prev.filter((l) => l.animeId !== animeId)); } catch {}
  };

  const handleExport = async () => {
    try {
      await SaveLibraryToFile();
    } catch (e) {
      console.error('Export failed:', e);
    }
  };

  const handleImport = async () => {
    try {
      const count = await ImportLibraryFromFile();
      if (count > 0) {
        await loadLibrary();
      }
    } catch (e) {
      console.error('Import failed:', e);
    }
  };

  const filteredLibrary = filter === 'all' ? library : library.filter((i) => i.status === filter);
  const statusCounts = STATUS_OPTIONS.reduce((acc, opt) => {
    acc[opt.value] = opt.value === 'all' ? library.length : library.filter((i) => i.status === opt.value).length;
    return acc;
  }, {} as Record<string, number>);

  if (loading) {
    return <div className="page-container" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}><div style={{ color: 'var(--text-muted)', fontSize: 14 }}>Loading...</div></div>;
  }

  return (
    <div className="page-container fade-in">
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 16px' }}>
        <div className="row-header" style={{ padding: 0 }}><IconLibrary size={22} style={{ color: 'var(--accent)' }} /> My Library</div>
        <div style={{ display: 'flex', gap: 6 }}>
          <button className="btn btn-outline" onClick={handleExport} style={{ fontSize: 12, height: 32, gap: 4 }}>
            <IconDownload size={14} /> Export
          </button>
          <button className="btn btn-outline" onClick={handleImport} style={{ fontSize: 12, height: 32, gap: 4 }}>
            <IconUpload size={14} /> Import
          </button>
        </div>
      </div>
      <div className="tab-bar">
        {STATUS_OPTIONS.map((opt) => (
          <button key={opt.value} className={`tab ${filter === opt.value ? 'active' : ''}`} onClick={() => setFilter(opt.value)}>
            {opt.label} ({statusCounts[opt.value] || 0})
          </button>
        ))}
      </div>
      {filteredLibrary.length === 0 ? (
        <div style={{ padding: '60px 16px', textAlign: 'center', color: 'var(--text-muted)' }}>
          {filter === 'all' ? 'Your library is empty. Search and add anime!' : `No anime in "${filter}"`}
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))', gap: 10, padding: '0 16px 16px' }}>
          {filteredLibrary.map((item) => (
            <div key={item.animeId} className="library-item">
              <img className="thumb" src={item.image} alt={item.title}
                onClick={() => navigate(`/anime/${item.animeId}`)}
                onError={(e) => { (e.target as HTMLImageElement).src = 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="80" height="120"%3E%3Crect fill="%232d2d3a" width="80" height="120"/%3E%3C/svg%3E'; }} />
              <div className="info">
                <div className="title" style={{ cursor: 'pointer' }} onClick={() => navigate(`/anime/${item.animeId}`)}>{item.title}</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 6 }}>
                  <span className="status-pill" style={{ background: STATUS_COLORS[item.status] || '#5c5f66', color: '#fff' }}>{item.status}</span>
                  {item.score > 0 && <span className="status-pill" style={{ background: 'var(--accent)', color: '#000' }}><IconStar size={10} /> {item.score}</span>}
                </div>
                <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 8 }}>{item.episodesWatch}/{item.totalEpisodes || '?'} episodes</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <select value={item.status} onChange={(e) => handleStatusChange(item.animeId, e.target.value)}
                    style={{ background: 'var(--bg-ep)', color: 'var(--text)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '3px 6px', fontSize: 11, outline: 'none' }}>
                    {STATUS_OPTIONS.filter((o) => o.value !== 'all').map((opt) => (
                      <option key={opt.value} value={opt.value}>{opt.label}</option>
                    ))}
                  </select>
                  <button className="btn-icon" onClick={() => handleRemove(item.animeId)} style={{ width: 28, height: 28 }}>
                    <IconTrash size={13} />
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
