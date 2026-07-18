import React, { useState, useEffect } from 'react';
import { IconSettings, IconExternalLink, IconRefresh, IconDownload, IconCheck, IconPlayerPlay } from '@tabler/icons-react';
import { EnsureTools, GetMALStatus, GetMALAuthURL, SyncToMAL, GetDownloads, GetAppVersion } from '../../wailsjs/go/main/App';

export default function Settings() {
  const [malStatus, setMalStatus] = useState('not_connected');
  const [downloads, setDownloads] = useState<string[]>([]);
  const [version, setVersion] = useState('');
  const [toolsInstalled, setToolsInstalled] = useState(false);

  useEffect(() => { loadSettings(); }, []);

  const loadSettings = async () => {
    try {
      const status = await GetMALStatus();
      setMalStatus(status as string);
      const files = await GetDownloads();
      setDownloads((files as any) || []);
      const ver = await GetAppVersion();
      setVersion(ver as string);
      try { await EnsureTools(); setToolsInstalled(true); } catch { setToolsInstalled(false); }
    } catch { /* ignore */ }
  };

  const handleMALConnect = async () => {
    try { const url = await GetMALAuthURL(); window.open(url as string, '_blank'); } catch {}
  };

  return (
    <div className="page-container fade-in">
      <div className="row-header"><IconSettings size={22} style={{ color: 'var(--accent)' }} /> Settings</div>
      <div style={{ maxWidth: 640, padding: '0 16px' }}>
        <div className="settings-card">
          <h3 style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <IconPlayerPlay size={18} style={{ color: 'var(--accent)' }} /> Player & Tools
          </h3>
          <p style={{ fontSize: 13, color: 'var(--text-sub)', marginBottom: 12 }}>
            VLC player is bundled with the application.
          </p>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <span className="badge" style={{
              background: toolsInstalled ? 'rgba(0,230,118,0.15)' : 'rgba(255,193,7,0.15)',
              color: toolsInstalled ? '#51cf66' : '#ffc107',
            }}>
              {toolsInstalled ? <IconCheck size={14} /> : <IconDownload size={14} />}
              {toolsInstalled ? 'VLC Found' : 'VLC Not Found'}
            </span>
          </div>
        </div>

        <div className="settings-card">
          <h3>MyAnimeList</h3>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: 13 }}>Account:</span>
              <span className="badge" style={{
                background: malStatus === 'connected' ? 'rgba(0,230,118,0.15)' : 'var(--bg-ep)',
                color: malStatus === 'connected' ? '#51cf66' : 'var(--text-muted)',
              }}>
                {malStatus === 'connected' ? 'Connected' : 'Not Connected'}
              </span>
            </div>
            {malStatus === 'not_connected' ? (
              <button className="btn btn-accent" onClick={handleMALConnect} style={{ fontSize: 12, height: 32 }}>
                <IconExternalLink size={14} /> Connect
              </button>
            ) : (
              <button className="btn btn-outline" onClick={() => SyncToMAL()} style={{ fontSize: 12, height: 32 }}>
                <IconRefresh size={14} /> Sync
              </button>
            )}
          </div>
        </div>

        <div className="settings-card">
          <h3>Downloads</h3>
          {downloads.length === 0 ? (
            <p style={{ fontSize: 13, color: 'var(--text-muted)' }}>No downloads yet</p>
          ) : (
            <div style={{ maxHeight: 160, overflow: 'auto' }}>
              {downloads.map((file, idx) => (
                <div key={idx} style={{ padding: '6px 10px', background: 'var(--bg-ep)', fontSize: 12, color: 'var(--text-sub)', marginBottom: 4 }}>{file}</div>
              ))}
            </div>
          )}
        </div>

        <div className="settings-card">
          <h3>About</h3>
          <p style={{ fontSize: 13, color: 'var(--text-muted)' }}>AnimoBox v{version || '1.0.0'} — Desktop anime client</p>
        </div>
      </div>
    </div>
  );
}
