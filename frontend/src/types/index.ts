export interface Anime {
  id: number;
  title: string;
  image: string;
  score: string;
  genres: string[];
  status: string;
  episodes: string;
  synopsis: string;
  aired: string;
  studios: string;
  type: string;
  duration: string;
  rating: string;
  source: string;
  episodeList: Episode[];
}

export interface Episode {
  id: string;
  number: string;
  title: string;
  image: string;
  duration: string;
  sub: string;
  dub: string;
}

export interface SearchResult {
  id: string;
  title: string;
  image: string;
  score: string;
  type: string;
  epsCount: string;
  status: string;
}

export interface TrendingAnime {
  id: string;
  title: string;
  image: string;
  rank: string;
  score: string;
  subs: string;
  dubs: string;
  type: string;
  eps: string;
}

export interface StreamLink {
  url: string;
  quality: string;
}

export interface StreamSource {
  server: string;
  type: string;
  links: StreamLink[];
}

export interface LibraryAnime {
  id: number;
  animeId: string;
  title: string;
  image: string;
  status: string;
  score: number;
  episodesWatch: number;
  totalEpisodes: string;
  updatedAt: string;
}
