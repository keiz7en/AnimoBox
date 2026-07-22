import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { IconArrowLeft, IconPlayerPlay, IconPlus, IconCheck, IconStar } from '@tabler/icons-react';
import { Anime, Episode } from '../types';
import { GetAnimeDetails, GetLibraryItem, AddToLibrary, RemoveFromLibrary } from '../../wailsjs/go/main/App';

export default function AnimeDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [anime, setAnime] = useState<Anime | null>(null);
  const [loading, setLoading] = useState(true);
  const [inLibrary, setInLibrary] = useState(false);

  useEffect(() => {
    if (id) {
      loadAnime(id);
      checkLibrary(id);
    }
  }, [id]);

  const loadAnime = async (animeId: string) => {
    setLoading(true);
    try {
      const data = await GetAnimeDetails(animeId);
      setAnime(data as any);
    } catch (e) {
      console.error('Failed to load anime:', e);
      setAnime(null);
    } finally {
      setLoading(false);
    }
  };

  const checkLibrary = async (animeId: string) => {
    try {
      const item = await GetLibraryItem(animeId);
      setInLibrary(!!item);
    } catch {
      setInLibrary(false);
    }
  };

  const handlePlayEpisode = (episode: Episode) => {
    navigate(`/watch/${episode.id}`, {
      state: {
        animeTitle: anime?.title || '',
        episodeNumber: episode.number || '',
        image: anime?.image || '',
      },
    });
  };

  const toggleLibrary = async () => {
    if (!anime) return;
    try {
      if (inLibrary) {
        await RemoveFromLibrary(id!);
        setInLibrary(false);
      } else {
        await AddToLibrary({
          id: 0,
          animeId: id!,
          title: anime.title,
          image: anime.image,
          status: 'watching',
          score: 0,
          episodesWatch: 0,
          totalEpisodes: anime.episodes || '?',
          updatedAt: '',
        } as any);
        setInLibrary(true);
      }
    } catch (err) {
      console.error('Failed to update library:', err);
    }
  };

  if (loading) {
    return (
      <div className="page-container" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <div style={{ color: 'var(--text-muted)', fontSize: 14 }}>Loading...</div>
      </div>
    );
  }

  if (!anime) {
    return (
      <div className="page-container" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <div style={{ color: 'var(--text-muted)', fontSize: 14 }}>Anime not found</div>
      </div>
    );
  }

  return (
    <div className="page-container fade-in">
      <button className="btn-icon" onClick={() => navigate(-1)} style={{ marginBottom: 12 }}>
        <IconArrowLeft size={18} />
      </button>

      <div className="detail-header">
        <div className="detail-poster">
          <img
            src={anime.image}
            alt={anime.title}
            onError={(e) => {
              (e.target as HTMLImageElement).src = 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="200" height="300"%3E%3Crect fill="%232d2d3a" width="200" height="300"/%3E%3C/svg%3E';
            }}
          />
        </div>
        <div className="detail-info">
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12, marginBottom: 8 }}>
            <h1 className="detail-title">{anime.title}</h1>
            <button
              className={`btn ${inLibrary ? 'btn-accent' : 'btn-outline'}`}
              onClick={toggleLibrary}
              style={{ flexShrink: 0, height: 36 }}
            >
              {inLibrary ? <IconCheck size={16} /> : <IconPlus size={16} />}
              {inLibrary ? 'In Library' : 'Add'}
            </button>
          </div>
          <div className="detail-meta">
            {anime.type && <span className="badge badge-accent">{anime.type}</span>}
            {anime.status && <span className="badge badge-muted">{anime.status}</span>}
            {anime.score && (
              <span className="badge badge-sub"><IconStar size={11} /> {anime.score}</span>
            )}
            {anime.episodes && <span className="badge badge-muted">{anime.episodes} Ep</span>}
          </div>
          {anime.genres && anime.genres.length > 0 && (
            <div className="detail-genres">
              {anime.genres.map((genre) => (
                <button key={genre} className="genre-chip" onClick={() => navigate(`/search?q=${encodeURIComponent(genre)}`)}>
                  {genre}
                </button>
              ))}
            </div>
          )}
          <div className="detail-info-grid">
            {anime.aired && <div className="detail-info-item"><div className="label">Aired</div><div className="value">{anime.aired}</div></div>}
            {anime.studios && <div className="detail-info-item"><div className="label">Studios</div><div className="value">{anime.studios}</div></div>}
            {anime.duration && <div className="detail-info-item"><div className="label">Duration</div><div className="value">{anime.duration}</div></div>}
            {anime.rating && <div className="detail-info-item"><div className="label">Rating</div><div className="value">{anime.rating}</div></div>}
          </div>
          {anime.synopsis && (
            <div style={{ marginTop: 12 }}>
              <div className="detail-info-item"><div className="label" style={{ marginBottom: 4 }}>Synopsis</div></div>
              <div className="detail-synopsis">{anime.synopsis}</div>
            </div>
          )}
        </div>
      </div>

      {anime.episodeList && anime.episodeList.length > 0 && (
        <div style={{ marginTop: 16 }}>
          <div className="row-header">Episodes ({anime.episodeList.length})</div>
          <div className="ep-row">
            {anime.episodeList.map((ep) => (
              <button key={ep.id} className="ep-btn" onClick={() => handlePlayEpisode(ep)}>
                {ep.number || '?'}
              </button>
            ))}
          </div>
        </div>
      )}

      {(!anime.episodeList || anime.episodeList.length === 0) && (
        <div style={{ marginTop: 16 }}>
          <div className="row-header">Streaming</div>
          <div style={{ padding: '0 16px' }}>
            <button
              className="btn btn-accent"
              onClick={() => navigate(`/watch/${anime.id}-1`, {
                state: {
                  animeTitle: anime.title,
                  episodeNumber: '1',
                  image: anime.image,
                },
              })}
              style={{ gap: 8 }}
            >
              <IconPlayerPlay size={16} /> Watch Episode 1
            </button>
            <div style={{ marginTop: 8, fontSize: 12, color: 'var(--text-muted)' }}>
              Find streaming sources for this anime.
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
