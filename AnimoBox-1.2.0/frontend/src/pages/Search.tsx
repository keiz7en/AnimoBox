import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import AnimeCard from '../components/AnimeCard';
import { SearchResult } from '../types';
import { IconSearch, IconX, IconStar, IconFlame, IconBolt, IconHeart, IconSword, IconGhost, IconLoader, IconTrophy, IconCalendar } from '@tabler/icons-react';
import { SearchAnime, GetTopAnime, GetSchedule } from '../../wailsjs/go/main/App';

const QUICK_SEARCHES = [
  { label: 'Naruto', icon: IconBolt },
  { label: 'One Piece', icon: IconStar },
  { label: 'Demon Slayer', icon: IconSword },
  { label: 'Jujutsu Kaisen', icon: IconGhost },
  { label: 'Attack on Titan', icon: IconFlame },
  { label: 'Spy x Family', icon: IconHeart },
  { label: 'My Hero Academia', icon: IconFlame },
  { label: 'Dragon Ball', icon: IconBolt },
  { label: 'Fullmetal Alchemist', icon: IconStar },
  { label: 'Death Note', icon: IconGhost },
];

const ALL_GENRES = [
  "Action","Adventure","Boys Love","Cars","Comedy","Dementia","Demons","Drama",
  "Fantasy","Game","Girls Love","Gourmet","Harem","Historical",
  "Horror","Isekai","Josei","Kids","Magic","Mahou Shoujo","Martial Arts","Mecha",
  "Military","Music","Mystery","Parody","Police","Psychological","Romance","Samurai",
  "School","Sci-Fi","Seinen","Shoujo","Shoujo Ai","Shounen","Shounen Ai",
  "Slice of Life","Space","Sports","Super Power","Supernatural","Suspense","Thriller","Vampire",
];

type BrowseTab = 'search' | 'top' | 'schedule';

export default function Search() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [query, setQuery] = useState(searchParams.get('q') || '');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);
  const [page, setPage] = useState(1);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const [activeTab, setActiveTab] = useState<BrowseTab>('search');
  const [topAnime, setTopAnime] = useState<SearchResult[]>([]);
  const [topPeriod, setTopPeriod] = useState('day');
  const [schedule, setSchedule] = useState<SearchResult[]>([]);
  const [loadingTop, setLoadingTop] = useState(false);
  const [loadingSchedule, setLoadingSchedule] = useState(false);
  const [showAllGenres, setShowAllGenres] = useState(false);

  const performSearch = useCallback(async (q: string, pageNum: number, append: boolean) => {
    const trimmed = q.trim();
    if (!trimmed) return;
    if (append) setLoadingMore(true);
    else setLoading(true);
    setSearched(true);
    try {
      const data = await SearchAnime(trimmed, pageNum);
      const list = (data as any) || [];
      if (append) setResults((prev) => [...prev, ...list]);
      else setResults(list);
      setHasMore(list.length >= 50);
    } catch (e) {
      console.error('Search failed:', e);
      if (!append) setResults([]);
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
  }, []);

  useEffect(() => {
    const q = searchParams.get('q');
    if (q) {
      setQuery(q);
      setResults([]);
      setPage(1);
      setHasMore(true);
      setActiveTab('search');
      performSearch(q, 1, false);
    }
  }, [searchParams, performSearch]);

  useEffect(() => {
    if (activeTab === 'top') {
      setLoadingTop(true);
      GetTopAnime(topPeriod).then((d) => { setTopAnime((d as any) || []); setLoadingTop(false); }).catch(() => { setTopAnime([]); setLoadingTop(false); });
    } else if (activeTab === 'schedule') {
      setLoadingSchedule(true);
      GetSchedule().then((d) => { setSchedule((d as any) || []); setLoadingSchedule(false); }).catch(() => { setSchedule([]); setLoadingSchedule(false); });
    }
  }, [activeTab, topPeriod]);

  const handleSearch = () => {
    if (query.trim()) {
      navigate(`/search?q=${encodeURIComponent(query.trim())}`);
    }
  };

  const handleQuickSearch = (q: string) => {
    setQuery(q);
    navigate(`/search?q=${encodeURIComponent(q)}`);
  };

  const handleGenreClick = (genre: string) => {
    setQuery(genre);
    navigate(`/search?q=${encodeURIComponent(genre)}`);
  };

  const loadMore = () => {
    const nextPage = page + 1;
    setPage(nextPage);
    performSearch(query, nextPage, true);
  };

  return (
    <div className="page-container fade-in">
      <div style={{ padding: '0 0 16px', display: 'flex', alignItems: 'center', gap: 8 }}>
        <div style={{ position: 'relative', flex: 1, maxWidth: 550 }}>
          <input
            className="input"
            placeholder="Search anime..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
            style={{ width: '100%', height: 42, fontSize: 14, paddingRight: query ? 36 : 12 }}
          />
          {query && (
            <button
              onClick={() => { setQuery(''); setResults([]); setSearched(false); navigate('/search'); }}
              style={{
                position: 'absolute', right: 6, top: '50%', transform: 'translateY(-50%)',
                background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer',
                display: 'flex', padding: 4,
              }}
            >
              <IconX size={16} />
            </button>
          )}
        </div>
        <button className="btn btn-accent" onClick={handleSearch} style={{ height: 42 }}>
          <IconSearch size={18} /> Search
        </button>
      </div>

      <div className="tab-bar" style={{ marginBottom: 16 }}>
        <button className={`tab ${activeTab === 'search' ? 'active' : ''}`} onClick={() => setActiveTab('search')}>
          <IconSearch size={14} style={{ verticalAlign: -2, marginRight: 4 }} /> Search
        </button>
        <button className={`tab ${activeTab === 'top' ? 'active' : ''}`} onClick={() => setActiveTab('top')}>
          <IconTrophy size={14} style={{ verticalAlign: -2, marginRight: 4 }} /> Top Anime
        </button>
        <button className={`tab ${activeTab === 'schedule' ? 'active' : ''}`} onClick={() => setActiveTab('schedule')}>
          <IconCalendar size={14} style={{ verticalAlign: -2, marginRight: 4 }} /> Schedule
        </button>
      </div>

      {activeTab === 'search' && (
        <>
          {!searched && (
            <div style={{ marginBottom: 20 }}>
              <div className="row-header">
                <IconBolt size={20} style={{ color: 'var(--accent)' }} />
                Quick Search
              </div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, padding: '0 16px 16px' }}>
                {QUICK_SEARCHES.map((item) => (
                  <button key={item.label} className="btn btn-outline" onClick={() => handleQuickSearch(item.label)} style={{ gap: 6 }}>
                    <item.icon size={14} />
                    {item.label}
                  </button>
                ))}
              </div>
            </div>
          )}

          {!searched && (
            <div style={{ marginBottom: 20 }}>
              <div className="row-header" style={{ justifyContent: 'space-between' }}>
                <span>Genres ({ALL_GENRES.length})</span>
                <button className="btn btn-outline" onClick={() => setShowAllGenres(!showAllGenres)} style={{ fontSize: 12, padding: '4px 10px' }}>
                  {showAllGenres ? 'Show Less' : 'Show All'}
                </button>
              </div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, padding: '0 16px 16px' }}>
                {(showAllGenres ? ALL_GENRES : ALL_GENRES.slice(0, 20)).map((genre) => (
                  <button key={genre} className="genre-chip" onClick={() => handleGenreClick(genre)}>
                    {genre}
                  </button>
                ))}
                {!showAllGenres && (
                  <button className="genre-chip" onClick={() => setShowAllGenres(true)} style={{ background: 'var(--accent)', color: '#000', fontWeight: 600 }}>
                    +{ALL_GENRES.length - 20} more
                  </button>
                )}
              </div>
            </div>
          )}

          {loading ? (
            <div style={{ padding: '60px 16px', textAlign: 'center', color: 'var(--text-muted)' }}>
              <IconLoader size={32} style={{ marginBottom: 8, animation: 'spin 1s linear infinite' }} />
              <div>Searching...</div>
            </div>
          ) : searched && results.length === 0 ? (
            <div style={{ padding: '60px 16px', textAlign: 'center', color: 'var(--text-muted)' }}>
              <div style={{ fontSize: 48, marginBottom: 8, opacity: 0.3 }}>&#128269;</div>
              <div>No results found for "{query}"</div>
            </div>
          ) : searched ? (
            <>
              <div className="row-header">
                <IconSearch size={20} style={{ color: 'var(--accent)' }} />
                Results for "{query}" ({results.length})
              </div>
              <div className="anime-grid">
                {results.map((anime) => (
                  <AnimeCard key={anime.id} anime={anime} />
                ))}
              </div>
              {hasMore && !loadingMore && (
                <div style={{ padding: '16px', textAlign: 'center' }}>
                  <button className="btn btn-outline" onClick={loadMore} style={{ minWidth: 160 }}>Load More</button>
                </div>
              )}
              {loadingMore && (
                <div style={{ padding: '16px', textAlign: 'center', color: 'var(--text-muted)' }}>
                  <IconLoader size={24} style={{ animation: 'spin 1s linear infinite' }} />
                </div>
              )}
            </>
          ) : null}
        </>
      )}

      {activeTab === 'top' && (
        <>
          <div className="tab-bar" style={{ marginBottom: 12, marginTop: -4 }}>
            {[{ key: 'day', label: 'Today', icon: IconFlame }, { key: 'week', label: 'This Week', icon: IconBolt }, { key: 'month', label: 'This Month', icon: IconStar }].map(({ key, label, icon: Icon }) => (
              <button key={key} className={`tab ${topPeriod === key ? 'active' : ''}`} onClick={() => setTopPeriod(key)}>
                <Icon size={14} style={{ verticalAlign: -2, marginRight: 4 }} />{label}
              </button>
            ))}
          </div>
          {loadingTop ? (
            <div style={{ padding: '60px 16px', textAlign: 'center', color: 'var(--text-muted)' }}>
              <IconLoader size={32} style={{ marginBottom: 8, animation: 'spin 1s linear infinite' }} />
              <div>Loading top anime...</div>
            </div>
          ) : (
            <>
              <div className="row-header">
                <IconTrophy size={20} style={{ color: 'var(--accent)' }} />
                Top Anime - {topPeriod === 'day' ? 'Today' : topPeriod === 'week' ? 'This Week' : 'This Month'}
              </div>
              <div className="anime-grid">
                {topAnime.map((anime) => (
                  <AnimeCard key={anime.id} anime={anime} showRank={true} />
                ))}
              </div>
            </>
          )}
        </>
      )}

      {activeTab === 'schedule' && (
        <>
          {loadingSchedule ? (
            <div style={{ padding: '60px 16px', textAlign: 'center', color: 'var(--text-muted)' }}>
              <IconLoader size={32} style={{ marginBottom: 8, animation: 'spin 1s linear infinite' }} />
              <div>Loading schedule...</div>
            </div>
          ) : (
            <>
              <div className="row-header">
                <IconCalendar size={20} style={{ color: 'var(--accent)' }} />
                Upcoming Episodes
              </div>
              <div className="anime-grid">
                {schedule.map((anime) => (
                  <AnimeCard key={anime.id} anime={anime} />
                ))}
              </div>
            </>
          )}
        </>
      )}
    </div>
  );
}
