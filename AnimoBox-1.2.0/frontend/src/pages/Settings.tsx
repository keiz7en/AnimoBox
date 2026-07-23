import React, { useState, useEffect } from 'react';
import { IconSettings, IconExternalLink, IconRefresh, IconDownload, IconCheck, IconPlayerPlay, IconBell, IconBellOff, IconFolder, IconWorld } from '@tabler/icons-react';
import { EnsureTools, GetMALStatus, GetMALAuthURL, SyncToMAL, GetDownloads, GetAppVersion, GetNotificationsEnabled, SetNotificationsEnabled, GetCustomVLCPath, BrowseVLCPath, TestNotification, LaunchVLC, OpenInBrowser } from '../../wailsjs/go/main/App';

export default function Settings() {
  const [malStatus, setMalStatus] = useState('not_connected');
  const [downloads, setDownloads] = useState<string[]>([]);
  const [version, setVersion] = useState('');
  const [toolsInstalled, setToolsInstalled] = useState(false);
  const [notifEnabled, setNotifEnabled] = useState(false);
  const [vlcPath, setVlcPath] = useState('');
  const [vlcCustomPath, setVlcCustomPath] = useState('');

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
      const notifVal = await GetNotificationsEnabled();
      setNotifEnabled(notifVal === 'true');
      const customPath = await GetCustomVLCPath();
      setVlcCustomPath(customPath as string || '');
    } catch { /* ignore */ }
  };

  const handleMALConnect = async () => {
    try { const url = await GetMALAuthURL(); window.open(url as string, '_blank'); } catch {}
  };

  const toggleNotifications = async () => {
    const newVal = !notifEnabled;
    try {
      await SetNotificationsEnabled(newVal);
      setNotifEnabled(newVal);
    } catch { /* ignore */ }
  };

  const handleTestNotification = async () => {
    try { await TestNotification(); } catch {}
  };

  const handleLaunchVLC = async () => {
    try { await LaunchVLC(); } catch {}
  };

  const handleOpenBrowser = async () => {
    try { await OpenInBrowser('https://github.com/keiz7en/AnimoBox'); } catch {}
  };

  const handleBrowseVLC = async () => {
    try {
      const path = await BrowseVLCPath();
      if (path) {
        setVlcCustomPath(path);
        setToolsInstalled(true);
      }
    } catch {}
  };

  const vlcFound = vlcCustomPath !== '' || toolsInstalled;

  return (
    <div className="page-container fade-in">
      <div className="row-header"><IconSettings size={22} style={{ color: 'var(--accent)' }} /> Settings</div>
      <div style={{ maxWidth: 640, padding: '0 16px' }}>
        <div className="settings-card">
          <h3 style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <IconPlayerPlay size={18} style={{ color: 'var(--accent)' }} /> Player & Tools
          </h3>
          <p style={{ fontSize: 13, color: 'var(--text-sub)', marginBottom: 12 }}>
            Select the VLC player location on your device.
          </p>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
            <span className="badge" style={{
              background: vlcFound ? 'rgba(0,230,118,0.15)' : 'rgba(255,193,7,0.15)',
              color: vlcFound ? '#51cf66' : '#ffc107',
            }}>
              {vlcFound ? <IconCheck size={14} /> : <IconDownload size={14} />}
              {vlcFound ? 'VLC Found' : 'VLC Not Found'}
            </span>
            {vlcCustomPath && (
              <span style={{ fontSize: 11, color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1 }}>
                {vlcCustomPath}
              </span>
            )}
          </div>
          <button className="btn btn-accent" onClick={handleBrowseVLC} style={{ fontSize: 12, height: 32, gap: 6 }}>
            <IconFolder size={14} /> Browse for VLC
          </button>
          <p style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 8 }}>
            Browse and select vlc.exe on your computer. Leave empty to use bundled VLC.
          </p>
          <div style={{ display: 'flex', gap: 8, marginTop: 12 }}>
            <button className="btn btn-accent" onClick={handleLaunchVLC} style={{ fontSize: 12, height: 32, gap: 6 }}>
              <IconPlayerPlay size={14} /> Run VLC
            </button>
            <button className="btn btn-outline" onClick={handleOpenBrowser} style={{ fontSize: 12, height: 32, gap: 6 }}>
              <IconWorld size={14} /> Open in Browser
            </button>
          </div>
        </div>

        <div className="settings-card">
          <h3 style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <IconBell size={18} style={{ color: 'var(--accent)' }} /> Notifications
          </h3>
          <p style={{ fontSize: 13, color: 'var(--text-sub)', marginBottom: 12 }}>
            Get notified when new episodes of your watching anime are released.
          </p>
          <div
            style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', cursor: 'pointer', padding: '10px 14px', background: 'var(--bg-ep)', borderRadius: 8 }}
            onClick={toggleNotifications}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              {notifEnabled ? <IconBell size={18} style={{ color: '#51cf66' }} /> : <IconBellOff size={18} style={{ color: 'var(--text-muted)' }} />}
              <div>
                <div style={{ fontSize: 14, color: 'var(--text)' }}>{notifEnabled ? 'Enabled' : 'Disabled'}</div>
                <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>Checks every 5 minutes for new episodes</div>
              </div>
            </div>
            <div style={{
              width: 44, height: 24, borderRadius: 12, padding: 2,
              background: notifEnabled ? '#51cf66' : 'var(--text-muted)',
              transition: 'background 0.2s',
            }}>
              <div style={{
                width: 20, height: 20, borderRadius: 10, background: '#fff',
                transform: notifEnabled ? 'translateX(20px)' : 'translateX(0)',
                transition: 'transform 0.2s',
              }} />
            </div>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 10 }}>
            <button className="btn btn-outline" onClick={handleTestNotification} style={{ fontSize: 12, height: 32, gap: 4 }}>
              <IconBell size={14} /> Test Notification
            </button>
          </div>
          <p style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 10 }}>
            Only checks anime marked as "Watching" in your library. Close the app to stop checking.
          </p>
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
