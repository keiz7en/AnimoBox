import React, { useState, useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import AnimeCard from '../components/AnimeCard';
import { SearchResult } from '../types';
import { IconSearch, IconX, IconStar, IconFlame, IconBolt, IconHeart, IconSword, IconGhost, IconLoader } from '@tabler/icons-react';
import { SearchAnime } from '../../wailsjs/go/main/App';

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
  { label: 'Sword Art Online', icon: IconSword },
  { label: 'Chainsaw Man', icon: IconSword },
];

const GENRES = [
  'Action', 'Adventure', 'Comedy', 'Drama', 'Fantasy',
  'Horror', 'Mystery', 'Romance', 'Sci-Fi',
  'Slice of Life', 'Sports', 'Supernatural', 'Thriller',
  'Mecha', 'Historical',
];

export default function Search() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [query, setQuery] = useState(searchParams.get('q') || '');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);

  useEffect(() => {
    const q = searchParams.get('q');
    if (q) {
      setQuery(q);
      performSearch(q);
    }
  }, [searchParams]);

  const performSearch = async (searchQuery: string) => {
    const trimmed = searchQuery.trim();
    if (!trimmed) return;
    setLoading(true);
    setSearched(true);
    try {
      const data = await SearchAnime(trimmed);
      setResults((data as any) || []);
    } catch (e) {
      console.error('Search failed:', e);
      setResults([]);
    } finally {
      setLoading(false);
    }
  };

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

      {!searched && (
        <div style={{ marginBottom: 20 }}>
          <div className="row-header">
            <IconBolt size={20} style={{ color: 'var(--accent)' }} />
            Quick Search
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, padding: '0 16px 16px' }}>
            {QUICK_SEARCHES.map((item) => (
              <button
                key={item.label}
                className="btn btn-outline"
                onClick={() => handleQuickSearch(item.label)}
                style={{ gap: 6 }}
              >
                <item.icon size={14} />
                {item.label}
              </button>
            ))}
          </div>
        </div>
      )}

      {!searched && (
        <div style={{ marginBottom: 20 }}>
          <div className="row-header">Genres</div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, padding: '0 16px 16px' }}>
            {GENRES.map((genre) => (
              <button
                key={genre}
                className="genre-chip"
                onClick={() => handleGenreClick(genre)}
              >
                {genre}
              </button>
            ))}
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
          <div style={{ fontSize: 12, marginTop: 8 }}>Try different keywords or check spelling</div>
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
        </>
      ) : null}
    </div>
  );
}
