import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import AnimeCard from '../components/AnimeCard';
import { TrendingAnime } from '../types';
import { IconSearch, IconFlame, IconClock, IconTrophy, IconCalendar, IconError404 } from '@tabler/icons-react';
import { GetTrending, GetRecentEpisodes, GetFinishedAiring, GetUpcoming } from '../../wailsjs/go/main/App';

type TabKey = 'trending' | 'recent' | 'finished' | 'upcoming';

export default function Home() {
  const navigate = useNavigate();
  const [trending, setTrending] = useState<TrendingAnime[]>([]);
  const [recentEpisodes, setRecentEpisodes] = useState<TrendingAnime[]>([]);
  const [finishedAiring, setFinishedAiring] = useState<TrendingAnime[]>([]);
  const [upcoming, setUpcoming] = useState<TrendingAnime[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [activeTab, setActiveTab] = useState<TabKey>('trending');
  const [loadedTabs, setLoadedTabs] = useState<Set<TabKey>>(new Set());

  useEffect(() => {
    loadTab('trending');
  }, []);

  const loadTab = async (tab: TabKey) => {
    if (loadedTabs.has(tab)) return;
    setLoading(true);
    setError(null);
    try {
      let data: TrendingAnime[] = [];
      switch (tab) {
        case 'trending': {
          const result = await GetTrending();
          data = (result as any) || [];
          setTrending(data);
          break;
        }
        case 'recent': {
          const result = await GetRecentEpisodes();
          data = (result as any) || [];
          setRecentEpisodes(data);
          break;
        }
        case 'finished': {
          const result = await GetFinishedAiring();
          data = (result as any) || [];
          setFinishedAiring(data);
          break;
        }
        case 'upcoming': {
          const result = await GetUpcoming();
          data = (result as any) || [];
          setUpcoming(data);
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
  };

  const handleTabChange = (tab: TabKey) => {
    setActiveTab(tab);
    loadTab(tab);
  };

  const handleSearch = () => {
    if (searchQuery.trim()) {
      navigate(`/search?q=${encodeURIComponent(searchQuery.trim())}`);
    }
  };

  const getDataForTab = (): TrendingAnime[] => {
    switch (activeTab) {
      case 'trending': return trending;
      case 'recent': return recentEpisodes;
      case 'finished': return finishedAiring;
      case 'upcoming': return upcoming;
    }
  };

  const tabData = getDataForTab();

  const TAB_CONFIG: { key: TabKey; label: string; icon: React.FC<any> }[] = [
    { key: 'trending', label: 'Top Anime', icon: IconFlame },
    { key: 'recent', label: 'Currently Airing', icon: IconClock },
    { key: 'finished', label: 'Finished Airing', icon: IconTrophy },
    { key: 'upcoming', label: 'Upcoming', icon: IconCalendar },
  ];

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

      <div className="tab-bar">
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
