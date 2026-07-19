import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import AnimeCard from '../components/AnimeCard';
import { TrendingAnime } from '../types';
import { IconSearch, IconFlame, IconClock, IconTrophy, IconCalendar, IconError404, IconRefresh, IconSparkles, IconSubtask, IconMicrophone } from '@tabler/icons-react';
import { GetTrending, GetRecentEpisodes, GetFinishedAiring, GetUpcoming, GetNewFinishedAiring } from '../../wailsjs/go/main/App';

type TabKey = 'latest' | 'trending' | 'newfinished' | 'finished' | 'upcoming';
type SubDubFilter = 'all' | 'sub' | 'dub';

const TAB_CONFIG: { key: TabKey; label: string; icon: React.FC<any> }[] = [
  { key: 'latest', label: 'Latest Episode', icon: IconClock },
  { key: 'trending', label: 'Trending', icon: IconFlame },
  { key: 'newfinished', label: 'New Finished Aired', icon: IconSparkles },
  { key: 'finished', label: 'Finished Airing', icon: IconTrophy },
  { key: 'upcoming', label: 'Upcoming', icon: IconCalendar },
];

export default function Home() {
  const navigate = useNavigate();
  const [trending, setTrending] = useState<TrendingAnime[]>([]);
  const [recentEpisodes, setRecentEpisodes] = useState<TrendingAnime[]>([]);
  const [finishedAiring, setFinishedAiring] = useState<TrendingAnime[]>([]);
  const [newFinishedAiring, setNewFinishedAiring] = useState<TrendingAnime[]>([]);
  const [upcoming, setUpcoming] = useState<TrendingAnime[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [activeTab, setActiveTab] = useState<TabKey>('latest');
  const [subDubFilter, setSubDubFilter] = useState<SubDubFilter>('all');
  const [loadedTabs, setLoadedTabs] = useState<Set<TabKey>>(new Set());

  const loadTab = useCallback(async (tab: TabKey, force?: boolean) => {
    if (!force && loadedTabs.has(tab)) return;
    setLoading(true);
    setError(null);
    try {
      switch (tab) {
        case 'trending': {
          const result = await GetTrending();
          setTrending((result as any) || []);
          break;
        }
        case 'latest': {
          const result = await GetRecentEpisodes();
          setRecentEpisodes((result as any) || []);
          break;
        }
        case 'finished': {
          const result = await GetFinishedAiring();
          setFinishedAiring((result as any) || []);
          break;
        }
        case 'newfinished': {
          const result = await GetNewFinishedAiring();
          setNewFinishedAiring((result as any) || []);
          break;
        }
        case 'upcoming': {
          const result = await GetUpcoming();
          setUpcoming((result as any) || []);
          break;
        }
      }
      setLoadedTabs((prev) => new Set(prev).add(tab));
    } catch (e: any) {
      console.error(`Failed to load ${tab}:`, e);
      setError(`Failed to load: ${e?.message || 'unknown error'}`);
    } finally {
      setLoading(false);
    }
  }, [loadedTabs]);

  useEffect(() => {
    loadTab('latest');
  }, []);

  useEffect(() => {
    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        loadTab(activeTab, true);
      }
    };
    document.addEventListener('visibilitychange', handleVisibility);
    return () => document.removeEventListener('visibilitychange', handleVisibility);
  }, [activeTab, loadTab]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.altKey && e.key === 'ArrowLeft') {
        e.preventDefault();
        navigate(-1);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [navigate]);

  const handleTabChange = (tab: TabKey) => {
    setActiveTab(tab);
    loadTab(tab, true);
  };

  const handleRefresh = () => {
    loadTab(activeTab, true);
  };

  const handleSearch = () => {
    if (searchQuery.trim()) {
      navigate(`/search?q=${encodeURIComponent(searchQuery.trim())}`);
    }
  };

  const getDataForTab = (): TrendingAnime[] => {
    let data: TrendingAnime[];
    switch (activeTab) {
      case 'latest': data = recentEpisodes; break;
      case 'trending': data = trending; break;
      case 'finished': data = finishedAiring; break;
      case 'newfinished': data = newFinishedAiring; break;
      case 'upcoming': data = upcoming; break;
      default: data = [];
    }

    if (activeTab === 'latest' && subDubFilter !== 'all') {
      data = data.filter((a) => {
        if (subDubFilter === 'sub') return a.subs !== '0';
        if (subDubFilter === 'dub') return a.dubs !== '0';
        return true;
      });
    }

    return data;
  };

  const tabData = getDataForTab();

  return (
    <div className="page-container fade-in">
      <div style={{ padding: '0 0 20px', display: 'flex', alignItems: 'center', gap: 8 }}>
        <input
          className="input"
          placeholder="Search anime..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          style={{ flex: 1, maxWidth: 450, height: 42, fontSize: 14 }}
        />
        <button className="btn btn-accent" onClick={handleSearch} style={{ height: 42 }}>
          <IconSearch size={18} /> Search
        </button>
      </div>

      {error && (
        <div style={{
          background: 'rgba(255,107,107,0.12)',
          border: '1px solid #ff6b6b',
          color: '#ff6b6b',
          padding: '10px 14px',
          marginBottom: 16,
          fontSize: 13,
        }}>
          {error}
        </div>
      )}

      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
        <div className="tab-bar" style={{ flex: 1 }}>
          {TAB_CONFIG.map(({ key, label, icon: Icon }) => (
            <button
              key={key}
              className={`tab ${activeTab === key ? 'active' : ''}`}
              onClick={() => handleTabChange(key)}
            >
              <Icon size={14} style={{ verticalAlign: -2, marginRight: 4 }} />
              {label}
            </button>
          ))}
        </div>
        <button className="btn-icon" onClick={handleRefresh} title="Refresh" disabled={loading}
          style={{ opacity: loading ? 0.5 : 1 }}>
          <IconRefresh size={16} className={loading ? 'spin' : ''} />
        </button>
      </div>

      {activeTab === 'latest' && (
        <div style={{ display: 'flex', gap: 6, marginBottom: 12 }}>
          {([
            { key: 'all' as SubDubFilter, label: 'All', icon: null },
            { key: 'sub' as SubDubFilter, label: 'Sub', icon: IconSubtask },
            { key: 'dub' as SubDubFilter, label: 'Dub', icon: IconMicrophone },
          ]).map(({ key, label, icon: SIcon }) => (
            <button
              key={key}
              className={`btn ${subDubFilter === key ? 'btn-accent' : 'btn-outline'}`}
              onClick={() => setSubDubFilter(key)}
              style={{ fontSize: 12, padding: '4px 12px', gap: 4 }}
            >
              {SIcon && <SIcon size={12} />}
              {label}
            </button>
          ))}
        </div>
      )}

      {loading ? (
        <div style={{ padding: '60px 16px', textAlign: 'center', color: 'var(--text-muted)' }}>
          Loading...
        </div>
      ) : tabData.length === 0 ? (
        <div style={{ padding: '60px 16px', textAlign: 'center', color: 'var(--text-muted)' }}>
          <IconError404 size={48} style={{ marginBottom: 8, opacity: 0.3 }} />
          <div>No data loaded. Check console for errors.</div>
        </div>
      ) : (
        <>
          <div className="anime-grid">
            {tabData.map((anime) => (
              <AnimeCard key={anime.id} anime={anime} showRank={activeTab !== 'upcoming'} />
            ))}
          </div>
        </>
      )}
    </div>
  );
}
