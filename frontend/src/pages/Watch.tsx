import React, { useState, useEffect } from 'react';
import { useParams, useNavigate, useLocation } from 'react-router-dom';
import { IconArrowLeft, IconPlayerPlay, IconExternalLink, IconRefresh, IconPlayerStop } from '@tabler/icons-react';
import { StreamSource } from '../types';
import { GetStreamURL, InitPlayer, PlayInMPV, MPVStop } from '../../wailsjs/go/main/App';

interface WatchState {
  animeTitle?: string;
  episodeNumber?: string;
  image?: string;
}

export default function Watch() {
  const { episodeId } = useParams<{ episodeId: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const state = (location.state as WatchState) || {};

  const [streamSources, setStreamSources] = useState<StreamSource[]>([]);
  const [currentSource, setCurrentSource] = useState<StreamSource | null>(null);
  const [currentUrl, setCurrentUrl] = useState('');
  const [loading, setLoading] = useState(true);
  const [playerStatus, setPlayerStatus] = useState<'idle' | 'starting' | 'playing' | 'error'>('idle');
  const [errorMsg, setErrorMsg] = useState('');
  const [playerReady, setPlayerReady] = useState(false);

  useEffect(() => {
    if (episodeId) loadStreamSources(episodeId);
    return () => { MPVStop().catch(() => {}); };
  }, [episodeId]);

  const loadStreamSources = async (epId: string) => {
    setLoading(true);
    setPlayerStatus('idle');
    setPlayerReady(false);
    try {
      const sources = await GetStreamURL(epId, state.animeTitle || '');
      setStreamSources((sources as any) || []);
      if (sources && sources.length > 0) {
        const directVideo = sources.find((s: any) => s.type === 'video');
        const first = directVideo || sources[0];
        setCurrentSource(first);
        if (first.links && first.links.length > 0) {
          setCurrentUrl(first.links[0].url);
        }
      }
    } catch (e) {
      console.error('Failed to load stream:', e);
    } finally {
      setLoading(false);
    }
  };

  const getEpisodeTitle = () => {
    if (state.animeTitle) {
      const ep = state.episodeNumber ? ` Episode ${state.episodeNumber}` : '';
      return `${state.animeTitle}${ep}`;
    }
    if (!episodeId) return 'Episode';
    return episodeId.replace(/-/g, ' ').split('?')[0];
  };

  const playInVLC = async (url: string) => {
    if (!url) return;
    if (currentSource?.type === 'embed') {
      openInBrowser(url);
      return;
    }
    setPlayerStatus('starting');
    setErrorMsg('');
    try {
      await InitPlayer('');
      setPlayerReady(true);
      await PlayInMPV(url);
      setPlayerStatus('playing');
    } catch (err: any) {
      setPlayerStatus('error');
      setErrorMsg(err?.message || 'Failed to start VLC. Make sure VLC is installed.');
    }
  };

  const openInBrowser = (url: string) => {
    window.open(url, '_blank');
  };

  const handleSourceChange = (idx: number) => {
    if (streamSources[idx]) {
      const source = streamSources[idx];
      setCurrentSource(source);
      if (source.links && source.links.length > 0) {
        setCurrentUrl(source.links[0].url);
        setPlayerStatus('idle');
        setPlayerReady(false);
      }
    }
  };

  if (loading) {
    return (
      <div className="page-container" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <div style={{ color: 'var(--text-muted)', fontSize: 14 }}>Loading streaming sources...</div>
      </div>
    );
  }

  const videoSources = streamSources.filter(s => s.type === 'video');
  const embedSources = streamSources.filter(s => s.type === 'embed');
  const streamingSources = streamSources.filter(s => s.type === 'streaming');
  const infoSources = streamSources.filter(s => s.type === 'info');

  return (
    <div className="page-container fade-in">
      <button className="btn-icon" onClick={() => navigate(-1)} style={{ marginBottom: 12 }}>
        <IconArrowLeft size={18} />
      </button>

      <div style={{ background: 'var(--bg-card)', border: '1px solid var(--border)', padding: 24, marginBottom: 16 }}>
        <div style={{ fontSize: 16, fontWeight: 700, color: 'var(--text)', marginBottom: 16 }}>{getEpisodeTitle()}</div>

        {/* Video Player Area */}
        <div style={{
          background: '#000', width: '100%', aspectRatio: '16/9', display: 'flex', flexDirection: 'column',
          alignItems: 'center', justifyContent: 'center', gap: 16, marginBottom: 16, borderRadius: 8, overflow: 'hidden',
        }}>
          {playerStatus === 'idle' && !currentSource && (
            <div style={{ color: 'var(--text-muted)', fontSize: 14, textAlign: 'center', padding: 20 }}>
              No streaming sources available
            </div>
          )}
          {playerStatus === 'idle' && currentSource && (
            <>
              <button className="btn btn-accent" onClick={() => playInVLC(currentUrl)}
                style={{ width: 80, height: 80, borderRadius: '50%', padding: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <IconPlayerPlay size={36} style={{ marginLeft: 4 }} />
              </button>
              <span style={{ color: 'var(--text-muted)', fontSize: 13 }}>
                {currentSource.type === 'embed' ? 'Open in Browser' : 'Play in VLC'}
              </span>
            </>
          )}
          {playerStatus === 'starting' && (
            <div style={{ color: 'var(--accent)', fontSize: 14, textAlign: 'center' }}>
              <div>Starting VLC player...</div>
              <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 4 }}>This may take a moment</div>
            </div>
          )}
          {playerStatus === 'playing' && (
            <div style={{ color: 'var(--accent)', fontSize: 14, textAlign: 'center' }}>
              <IconPlayerPlay size={48} style={{ marginBottom: 8 }} />
              <div>Playing in VLC window</div>
              <button className="btn btn-outline" onClick={() => { MPVStop(); setPlayerStatus('idle'); setPlayerReady(false); }} style={{ marginTop: 12 }}>
                <IconPlayerStop size={14} /> Stop
              </button>
            </div>
          )}
          {playerStatus === 'error' && (
            <div style={{ color: '#ff6b6b', fontSize: 13, textAlign: 'center', padding: 20 }}>
              <div style={{ marginBottom: 8 }}>{errorMsg}</div>
              <div style={{ display: 'flex', gap: 8, justifyContent: 'center' }}>
                <button className="btn btn-outline" onClick={() => playInVLC(currentUrl)}>Retry VLC</button>
                <button className="btn btn-accent" onClick={() => openInBrowser(currentUrl)}>
                  <IconExternalLink size={14} /> Open in Browser
                </button>
              </div>
            </div>
          )}
        </div>

        {/* Source Controls */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          <button className="btn-icon" onClick={() => loadStreamSources(episodeId || '')} title="Refresh">
            <IconRefresh size={14} />
          </button>

          {playerReady && (
            <>
              <button className="btn-icon" onClick={() => openInBrowser(currentUrl)} title="Open in browser">
                <IconExternalLink size={14} />
              </button>
            </>
          )}
        </div>
      </div>

      {/* Video Sources (Direct .mp4) */}
      {videoSources.length > 0 && (
        <div style={{ background: 'var(--bg-card)', border: '1px solid var(--border)', padding: 16, marginBottom: 12 }}>
          <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-muted)', marginBottom: 8 }}>
            DIRECT VIDEO (VLC)
          </div>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {videoSources.map((source, i) => (
              <button
                key={i}
                className={`btn ${currentSource === source ? 'btn-accent' : 'btn-outline'}`}
                onClick={() => handleSourceChange(streamSources.indexOf(source))}
                style={{ fontSize: 12 }}
              >
                <IconPlayerPlay size={14} /> {source.server} ({source.links[0]?.quality})
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Embed Sources */}
      {embedSources.length > 0 && (
        <div style={{ background: 'var(--bg-card)', border: '1px solid var(--border)', padding: 16, marginBottom: 12 }}>
          <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-muted)', marginBottom: 8 }}>
            EMBED PLAYER
          </div>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {embedSources.map((source, i) => (
              <button
                key={i}
                className={`btn ${currentSource === source ? 'btn-accent' : 'btn-outline'}`}
                onClick={() => openInBrowser(source.links[0]?.url || '')}
                style={{ fontSize: 12 }}
              >
                <IconExternalLink size={14} /> {source.server}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Streaming Services */}
      {streamingSources.length > 0 && (
        <div style={{ background: 'var(--bg-card)', border: '1px solid var(--border)', padding: 16, marginBottom: 12 }}>
          <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-muted)', marginBottom: 8 }}>
            STREAMING SERVICES
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 8 }}>
            {streamingSources.map((source, i) => (
              <button
                key={i}
                className="btn btn-outline"
                onClick={() => openInBrowser(source.links[0]?.url || '')}
                style={{ justifyContent: 'flex-start', textTransform: 'none', fontSize: 12 }}
              >
                <IconExternalLink size={14} /> {source.server}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Info Sources */}
      {infoSources.length > 0 && (
        <div style={{ background: 'var(--bg-card)', border: '1px solid var(--border)', padding: 16 }}>
          <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-muted)', marginBottom: 8 }}>
            MORE INFO
          </div>
          <button className="btn btn-outline" onClick={() => openInBrowser(infoSources[0].links[0]?.url || '')} style={{ fontSize: 12 }}>
            <IconExternalLink size={14} /> View on MyAnimeList
          </button>
        </div>
      )}
    </div>
  );
}
