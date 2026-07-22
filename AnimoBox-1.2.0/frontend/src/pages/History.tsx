import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { IconHistory, IconTrash } from '@tabler/icons-react';
import { HistoryItem } from '../types';
import { GetHistory, ClearHistory } from '../../wailsjs/go/main/App';

export default function History() {
  const navigate = useNavigate();
  const [history, setHistory] = useState<HistoryItem[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => { loadHistory(); }, []);

  const loadHistory = async () => {
    setLoading(true);
    try {
      const data = await GetHistory();
      setHistory((data as any) || []);
    } catch { setHistory([]); } finally { setLoading(false); }
  };

  const handleClear = async () => {
    try {
      await ClearHistory();
      setHistory([]);
    } catch {}
  };

  if (loading) {
    return (
      <div className="page-container" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <div style={{ color: 'var(--text-muted)', fontSize: 14 }}>Loading...</div>
      </div>
    );
  }

  return (
    <div className="page-container fade-in">
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 16px' }}>
        <div className="row-header" style={{ padding: 0 }}>
          <IconHistory size={22} style={{ color: 'var(--accent)' }} /> Watch History
        </div>
        {history.length > 0 && (
          <button className="btn btn-outline" onClick={handleClear} style={{ fontSize: 12, height: 32, gap: 4 }}>
            <IconTrash size={14} /> Clear All
          </button>
        )}
      </div>

      {history.length === 0 ? (
        <div style={{ padding: '60px 16px', textAlign: 'center', color: 'var(--text-muted)' }}>
          <IconHistory size={48} style={{ marginBottom: 8, opacity: 0.3 }} />
          <div>No watch history yet.</div>
          <div style={{ fontSize: 12, marginTop: 4 }}>Episodes you watch will appear here.</div>
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))', gap: 10, padding: '0 16px 16px' }}>
          {history.map((item) => (
            <div key={item.id} className="library-item" onClick={() => navigate(`/anime/${item.animeId}`)} style={{ cursor: 'pointer' }}>
              <img className="thumb" src={item.image} alt={item.title}
                onError={(e) => { (e.target as HTMLImageElement).src = 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="80" height="120"%3E%3Crect fill="%232d2d3a" width="80" height="120"/%3E%3C/svg%3E'; }} />
              <div className="info">
                <div className="title">{item.title}</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 6 }}>
                  {item.episodeNumber && (
                    <span className="status-pill" style={{ background: 'var(--accent)', color: '#000' }}>
                      Ep {item.episodeNumber}
                    </span>
                  )}
                </div>
                <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>
                  {item.watchedAt ? new Date(item.watchedAt + 'Z').toLocaleString() : ''}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
