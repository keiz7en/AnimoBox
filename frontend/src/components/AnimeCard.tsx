import React from 'react';
import { useNavigate } from 'react-router-dom';
import { TrendingAnime, SearchResult } from '../types';

interface AnimeCardProps {
  anime: TrendingAnime | SearchResult;
  showRank?: boolean;
}

export default function AnimeCard({ anime, showRank = false }: AnimeCardProps) {
  const navigate = useNavigate();
  const title = 'title' in anime ? anime.title : '';
  const image = 'image' in anime ? anime.image : '';
  const score = 'score' in anime ? anime.score : '';
  const type = 'type' in anime ? anime.type : '';
  const rank = 'rank' in anime ? anime.rank : '';
  const epsCount = 'epsCount' in anime ? anime.epsCount : '';
  const subs = 'subs' in anime ? (anime as TrendingAnime).subs : '';
  const dubs = 'dubs' in anime ? (anime as TrendingAnime).dubs : '';

  return (
    <div
      className="anime-card"
      onClick={() => navigate(`/anime/${anime.id}`)}
    >
      <img
        className="poster"
        src={image}
        alt={title}
        loading="lazy"
        onError={(e) => {
          (e.target as HTMLImageElement).src = 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="200" height="300"%3E%3Crect fill="%232d2d3a" width="200" height="300"/%3E%3Ctext fill="%235c5f66" font-family="sans-serif" font-size="13" x="50%25" y="50%25" dominant-baseline="middle" text-anchor="middle"%3ENo Image%3C/text%3E%3C/svg%3E';
        }}
      />

      {showRank && rank && (
        <div className="badge-rank">#{rank}</div>
      )}

      {score && (
        <div className="badge-top">{score}</div>
      )}

      {type && (
        <div
          className="badge-sub"
          style={{ position: 'absolute', bottom: 6, right: 6, fontSize: 10 }}
        >
          {type}
        </div>
      )}

      {epsCount && (
        <div
          className="badge-muted"
          style={{ position: 'absolute', bottom: 6, left: 6, fontSize: 10 }}
        >
          {epsCount}
        </div>
      )}

      {subs && subs !== '0' && (
        <div style={{
          position: 'absolute', top: 6, left: 6, fontSize: 9, fontWeight: 700,
          background: 'rgba(0,230,118,0.85)', color: '#000', padding: '1px 5px',
          borderRadius: 3, lineHeight: '14px',
        }}>
          SUB {subs}
        </div>
      )}

      {dubs && dubs !== '0' && (
        <div style={{
          position: 'absolute', top: 6, right: 6, fontSize: 9, fontWeight: 700,
          background: 'rgba(41,182,246,0.85)', color: '#000', padding: '1px 5px',
          borderRadius: 3, lineHeight: '14px',
        }}>
          DUB {dubs}
        </div>
      )}

      <div className="card-info">
        <div className="card-title">{title}</div>
        {(score || type) && (
          <div className="card-meta">
            {score && <span className="score">{score}</span>}
            {type && <span>{type}</span>}
          </div>
        )}
      </div>
    </div>
  );
}
