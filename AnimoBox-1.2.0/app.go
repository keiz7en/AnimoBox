package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	sruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/glebarez/go-sqlite"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/oauth2"
)

type App struct {
	ctx         context.Context
	db          *sql.DB
	notifStopCh chan struct{}
}

type Anime struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Image       string    `json:"image"`
	Score       string    `json:"score"`
	Genres      []string  `json:"genres"`
	Status      string    `json:"status"`
	Episodes    string    `json:"episodes"`
	Synopsis    string    `json:"synopsis"`
	Aired       string    `json:"aired"`
	Studios     string    `json:"studios"`
	Type        string    `json:"type"`
	Duration    string    `json:"duration"`
	Rating      string    `json:"rating"`
	Source      string    `json:"source"`
	EpisodeList []Episode `json:"episodeList"`
}

type Episode struct {
	ID       string `json:"id"`
	Number   string `json:"number"`
	Title    string `json:"title"`
	Image    string `json:"image"`
	Duration string `json:"duration"`
	Sub      string `json:"sub"`
	Dub      string `json:"dub"`
}

type SearchResult struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Image    string `json:"image"`
	Score    string `json:"score"`
	Type     string `json:"type"`
	EpsCount string `json:"epsCount"`
	Status   string `json:"status"`
	Rank     int    `json:"rank,omitempty"`
	NextEp   string `json:"nextEp,omitempty"`
	NextTime string `json:"nextTime,omitempty"`
	AiringAt int64  `json:"airingAt,omitempty"`
}

type TrendingAnime struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Image     string `json:"image"`
	Rank      string `json:"rank"`
	Score     string `json:"score"`
	Subs      string `json:"subs"`
	Dubs      string `json:"dubs"`
	Type      string `json:"type"`
	Eps       string `json:"eps"`
	AiringAt  int64  `json:"airingAt"`
	NextEp    int    `json:"nextEp"`
	Status    string `json:"status"`
	IsNew     bool   `json:"isNew"`
}

type StreamLink struct {
	URL     string `json:"url"`
	Quality string `json:"quality"`
}

type StreamSource struct {
	Server string       `json:"server"`
	Type   string       `json:"type"`
	Links  []StreamLink `json:"links"`
}

type AnimeHeavenSearchResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Image string `json:"image"`
	Eps   string `json:"eps"`
}

type AnimeHeavenEpisode struct {
	VideoURL string `json:"videoUrl"`
	Quality  string `json:"quality"`
}

type LibraryAnime struct {
	ID               int    `json:"id"`
	AnimeID          string `json:"animeId"`
	Title            string `json:"title"`
	Image            string `json:"image"`
	Status           string `json:"status"`
	Score            int    `json:"score"`
	EpisodesWatch    int    `json:"episodesWatch"`
	TotalEpisodes    string `json:"totalEpisodes"`
	LastKnownEpisodes int   `json:"lastKnownEpisodes"`
	UpdatedAt        string `json:"updatedAt"`
}

type HistoryItem struct {
	ID             int    `json:"id"`
	AnimeID        string `json:"animeId"`
	Title          string `json:"title"`
	Image          string `json:"image"`
	EpisodeNumber  string `json:"episodeNumber"`
	WatchedAt      string `json:"watchedAt"`
}

const anilistURL = "https://graphql.anilist.co"

var (
	anilistHTTPClient = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}
	anilistMu      sync.Mutex
	anilistLastReq time.Time
)

const anilistRateLimit = 300 * time.Millisecond

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.initDB()
	a.startNotificationChecker()
	wruntime.LogInfo(ctx, "AnimoBox started successfully")
}

func (a *App) shutdown(ctx context.Context) {
	a.stopNotificationChecker()
	a.stopMPV()
	if a.db != nil {
		a.db.Close()
	}
}

func (a *App) initDB() {
	homeDir, _ := os.UserHomeDir()
	dbPath := filepath.Join(homeDir, ".animobox", "animobox.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	var err error
	a.db, err = sql.Open("sqlite", dbPath+"?_journal_mode=WAL")
	if err != nil {
		log.Printf("Failed to open database: %v", err)
		return
	}

	schema := `
	CREATE TABLE IF NOT EXISTS library (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		anime_id TEXT UNIQUE NOT NULL,
		title TEXT NOT NULL,
		image TEXT DEFAULT '',
		status TEXT DEFAULT 'plantowatch',
		score INTEGER DEFAULT 0,
		episodes_watch INTEGER DEFAULT 0,
		total_episodes TEXT DEFAULT '?',
		last_known_episodes INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		anime_id TEXT NOT NULL,
		title TEXT NOT NULL,
		image TEXT DEFAULT '',
		episode_number TEXT DEFAULT '',
		watched_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err = a.db.Exec(schema)
	if err != nil {
		log.Printf("Failed to create tables: %v", err)
	}

	// Add indexes for faster queries
	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_history_watched_at ON history(watched_at DESC)")
	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_history_anime_id ON history(anime_id)")
	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_library_status ON library(status)")
	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_library_updated_at ON library(updated_at DESC)")

	// Trim old history on startup
	a.db.Exec("DELETE FROM history WHERE id NOT IN (SELECT id FROM history ORDER BY watched_at DESC LIMIT 500)")

	// Migration: add last_known_episodes column to existing databases
	a.db.Exec("ALTER TABLE library ADD COLUMN last_known_episodes INTEGER DEFAULT 0")
}

func (a *App) anilistQuery(query string, variables map[string]interface{}, result interface{}) error {
	anilistMu.Lock()
	wait := anilistRateLimit - time.Since(anilistLastReq)
	if wait > 0 {
		time.Sleep(wait)
	}
	anilistLastReq = time.Now()
	anilistMu.Unlock()

	body := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", anilistURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AnimoBox/1.0")

	resp, err := anilistHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		time.Sleep(2 * time.Second)
		return a.anilistQuery(query, variables, result)
	}

	if resp.StatusCode != 200 {
		var errResp struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if len(errResp.Errors) > 0 {
			return fmt.Errorf("AniList: %s", errResp.Errors[0].Message)
		}
		return fmt.Errorf("AniList returned status %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

const trendingQuery = `
query ($page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    media(type: ANIME, sort: POPULARITY_DESC, isAdult: false) {
      id
      title { romaji english }
      coverImage { large color }
      averageScore
      format
      episodes
      status
      nextAiringEpisode { airingAt episode }
    }
  }
}`

const searchQuery = `
query ($search: String, $page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    pageInfo {
      total
      lastPage
    }
    media(search: $search, type: ANIME, isAdult: false) {
      id
      title { romaji english }
      coverImage { large color }
      averageScore
      format
      episodes
      status
    }
  }
}`

const genreQuery = `
query ($genre: String, $page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    pageInfo {
      total
      lastPage
    }
    media(genre: $genre, type: ANIME, sort: POPULARITY_DESC, isAdult: false) {
      id
      title { romaji english }
      coverImage { large color }
      averageScore
      format
      episodes
      status
    }
  }
}`

const animeDetailQuery = `
query ($id: Int) {
  Media(id: $id, type: ANIME) {
    id
    title { romaji english }
    coverImage { large color }
    bannerImage
    averageScore
    genres
    status
    episodes
    description
    startDate { year month day }
    endDate { year month day }
    format
    duration
    source
    studios(isMain: true) { nodes { name } }
    relations {
      edges {
        relationType
        node {
          id
          title { romaji english }
          format
          episodes
        }
      }
    }
  }
}`

const seasonQuery = `
query ($season: MediaSeason, $seasonYear: Int, $page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    media(type: ANIME, season: $season, seasonYear: $seasonYear, sort: POPULARITY_DESC, status_in: [RELEASING, NOT_YET_RELEASED], isAdult: false) {
      id
      title { romaji english }
      coverImage { large color }
      averageScore
      format
      episodes
      status
      genres
      nextAiringEpisode { episode airingAt }
    }
  }
}`

const finishedQuery = `
query ($page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    media(type: ANIME, status: FINISHED, sort: UPDATED_AT_DESC, isAdult: false) {
      id
      title { romaji english }
      coverImage { large color }
      averageScore
      format
      episodes
      status
      endDate { year month day }
    }
  }
}`

const upcomingQuery = `
query ($page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    media(type: ANIME, status: NOT_YET_RELEASED, sort: POPULARITY_DESC, isAdult: false) {
      id
      title { romaji english }
      coverImage { large color }
      averageScore
      format
      episodes
      status
      startDate { year month day }
      nextAiringEpisode { episode airingAt }
    }
  }
}`

const newFinishedQuery = `
query ($page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    media(type: ANIME, status: FINISHED, sort: END_DATE_DESC, onList: false, isAdult: false) {
      id
      title { romaji english }
      coverImage { large color }
      averageScore
      format
      episodes
      status
      endDate { year month day }
    }
  }
}`

func anilistTitle(t struct {
	Romaji  *string `json:"romaji"`
	English *string `json:"english"`
}) string {
	if t.English != nil && *t.English != "" {
		return *t.English
	}
	if t.Romaji != nil && *t.Romaji != "" {
		return *t.Romaji
	}
	return "Unknown"
}

func anilistImage(img struct {
	Large *string `json:"large"`
}) string {
	if img.Large != nil {
		return *img.Large
	}
	return ""
}

func (a *App) GetTrending() ([]TrendingAnime, error) {
	type mediaItem struct {
		ID             int    `json:"id"`
		Title          struct {
			Romaji  *string `json:"romaji"`
			English *string `json:"english"`
		} `json:"title"`
		CoverImage struct {
			Large *string `json:"large"`
		} `json:"coverImage"`
		AverageScore *int    `json:"averageScore"`
		Format      *string `json:"format"`
		Episodes    *int    `json:"episodes"`
	}

	var resp struct {
		Data struct {
			Page struct {
				Media []mediaItem `json:"media"`
			} `json:"Page"`
		} `json:"data"`
	}

	err := a.anilistQuery(trendingQuery, map[string]interface{}{
		"page":    1,
		"perPage": 50,
	}, &resp)
	if err != nil {
		return nil, err
	}

	var results []TrendingAnime
	for i, item := range resp.Data.Page.Media {
		title := anilistTitle(item.Title)
		img := anilistImage(item.CoverImage)
		score := ""
		if item.AverageScore != nil {
			score = fmt.Sprintf("%.1f", float64(*item.AverageScore)/10.0)
		}
		eps := "?"
		if item.Episodes != nil {
			eps = strconv.Itoa(*item.Episodes)
		}
		typ := ""
		if item.Format != nil {
			typ = *item.Format
		}

		results = append(results, TrendingAnime{
			ID:    strconv.Itoa(item.ID),
			Title: title,
			Image: img,
			Rank:  strconv.Itoa(i + 1),
			Score: score,
			Type:  typ,
			Eps:   eps,
		})
	}

	return results, nil
}

var allGenres = []string{
	"Action", "Adventure", "Boys Love", "Cars", "Comedy", "Dementia",
	"Demons", "Drama", "Fantasy", "Game",
	"Girls Love", "Gourmet", "Harem", "Historical", "Horror", "Isekai",
	"Josei", "Kids", "Magic", "Mahou Shoujo", "Martial Arts", "Mecha",
	"Military", "Music", "Mystery", "Parody", "Police", "Psychological",
	"Romance", "Samurai", "School", "Sci-Fi", "Seinen", "Shoujo",
	"Shoujo Ai", "Shounen", "Shounen Ai", "Slice of Life", "Space",
	"Sports", "Super Power", "Supernatural", "Suspense", "Thriller",
	"Vampire",
}

func isGenre(query string) string {
	for _, g := range allGenres {
		if strings.EqualFold(g, query) {
			return g
		}
	}
	return ""
}

func (a *App) GetGenres() []string {
	return allGenres
}

func (a *App) SearchAnime(query string, page int) ([]SearchResult, error) {
	type mediaItem struct {
		ID             int    `json:"id"`
		Title          struct {
			Romaji  *string `json:"romaji"`
			English *string `json:"english"`
		} `json:"title"`
		CoverImage struct {
			Large *string `json:"large"`
		} `json:"coverImage"`
		AverageScore *int    `json:"averageScore"`
		Format      *string `json:"format"`
		Episodes    *int    `json:"episodes"`
		Status      *string `json:"status"`
	}

	if page < 1 {
		page = 1
	}

	var resp struct {
		Data struct {
			Page struct {
				PageInfo struct {
					Total    int `json:"total"`
					LastPage int `json:"lastPage"`
				} `json:"pageInfo"`
				Media []mediaItem `json:"media"`
			} `json:"Page"`
		} `json:"data"`
	}

	genre := isGenre(strings.TrimSpace(query))
	var err error
	if genre != "" {
		err = a.anilistQuery(genreQuery, map[string]interface{}{
			"genre":   genre,
			"page":    page,
			"perPage": 50,
		}, &resp)
	} else {
		err = a.anilistQuery(searchQuery, map[string]interface{}{
			"search":  strings.TrimSpace(query),
			"page":    page,
			"perPage": 50,
		}, &resp)
	}
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, item := range resp.Data.Page.Media {
		title := anilistTitle(item.Title)
		img := anilistImage(item.CoverImage)
		score := ""
		if item.AverageScore != nil {
			score = fmt.Sprintf("%.1f", float64(*item.AverageScore)/10.0)
		}
		epsCount := "?"
		if item.Episodes != nil {
			epsCount = strconv.Itoa(*item.Episodes)
		}
		typ := ""
		if item.Format != nil {
			typ = *item.Format
		}
		status := ""
		if item.Status != nil {
			status = *item.Status
		}

		results = append(results, SearchResult{
			ID:       strconv.Itoa(item.ID),
			Title:    title,
			Image:    img,
			Score:    score,
			Type:     typ,
			EpsCount: epsCount,
			Status:   status,
		})
	}

	return results, nil
}

const topAnimeQuery = `
query ($sort: [MediaSort], $page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    media(type: ANIME, sort: $sort, status_in: [RELEASING, FINISHED], isAdult: false) {
      id
      title { romaji english }
      coverImage { large color }
      averageScore
      format
      episodes
      status
      trending
    }
  }
}`

func (a *App) GetTopAnime(period string) ([]SearchResult, error) {
	type mediaItem struct {
		ID             int    `json:"id"`
		Title          struct {
			Romaji  *string `json:"romaji"`
			English *string `json:"english"`
		} `json:"title"`
		CoverImage struct {
			Large *string `json:"large"`
		} `json:"coverImage"`
		AverageScore *int    `json:"averageScore"`
		Format      *string `json:"format"`
		Episodes    *int    `json:"episodes"`
		Status      *string `json:"status"`
		Trending    int     `json:"trending"`
	}

	sort := "SCORE_DESC"
	switch period {
	case "day":
		sort = "TRENDING_DESC"
	case "week":
		sort = "POPULARITY_DESC"
	case "month":
		sort = "FAVORITES_DESC"
	}

	var resp struct {
		Data struct {
			Page struct {
				Media []mediaItem `json:"media"`
			} `json:"Page"`
		} `json:"data"`
	}

	err := a.anilistQuery(topAnimeQuery, map[string]interface{}{
		"sort":    sort,
		"page":    1,
		"perPage": 50,
	}, &resp)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	for i, item := range resp.Data.Page.Media {
		title := anilistTitle(item.Title)
		img := anilistImage(item.CoverImage)
		score := ""
		if item.AverageScore != nil {
			score = fmt.Sprintf("%.1f", float64(*item.AverageScore)/10.0)
		}
		epsCount := "?"
		if item.Episodes != nil {
			epsCount = strconv.Itoa(*item.Episodes)
		}
		typ := ""
		if item.Format != nil {
			typ = *item.Format
		}
		status := ""
		if item.Status != nil {
			status = *item.Status
		}

		results = append(results, SearchResult{
			ID:       strconv.Itoa(item.ID),
			Title:    title,
			Image:    img,
			Score:    score,
			Type:     typ,
			EpsCount: epsCount,
			Status:   status,
			Rank:     i + 1,
		})
	}

	return results, nil
}

const scheduleQuery = `
query ($page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    media(type: ANIME, status: RELEASING, sort: POPULARITY_DESC, isAdult: false) {
      id
      title { romaji english }
      coverImage { large color }
      averageScore
      format
      episodes
      status
      airingSchedule(notYetAired: true, perPage: 1) {
        nodes {
          airingAt
          episode
        }
      }
    }
  }
}`

func (a *App) GetSchedule() ([]SearchResult, error) {
	type airNode struct {
		AiringAt int `json:"airingAt"`
		Episode  int `json:"episode"`
	}
	type mediaItem struct {
		ID             int    `json:"id"`
		Title          struct {
			Romaji  *string `json:"romaji"`
			English *string `json:"english"`
		} `json:"title"`
		CoverImage struct {
			Large *string `json:"large"`
		} `json:"coverImage"`
		AverageScore *int    `json:"averageScore"`
		Format      *string `json:"format"`
		Episodes    *int    `json:"episodes"`
		Status      *string `json:"status"`
		AiringSchedule struct {
			Nodes []airNode `json:"nodes"`
		} `json:"airingSchedule"`
	}

	var resp struct {
		Data struct {
			Page struct {
				Media []mediaItem `json:"media"`
			} `json:"Page"`
		} `json:"data"`
	}

	err := a.anilistQuery(scheduleQuery, map[string]interface{}{
		"page":    1,
		"perPage": 50,
	}, &resp)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, item := range resp.Data.Page.Media {
		title := anilistTitle(item.Title)
		img := anilistImage(item.CoverImage)
		score := ""
		if item.AverageScore != nil {
			score = fmt.Sprintf("%.1f", float64(*item.AverageScore)/10.0)
		}
		epsCount := "?"
		if item.Episodes != nil {
			epsCount = strconv.Itoa(*item.Episodes)
		}
		typ := ""
		if item.Format != nil {
			typ = *item.Format
		}
		status := ""
		if item.Status != nil {
			status = *item.Status
		}

		nextEp := ""
		nextTime := ""
		airingAtVal := int64(0)
		if len(item.AiringSchedule.Nodes) > 0 {
			n := item.AiringSchedule.Nodes[0]
			nextEp = strconv.Itoa(n.Episode)
			t := time.Unix(int64(n.AiringAt), 0)
			nextTime = t.Format("Mon Jan 2 3:04 PM")
			airingAtVal = int64(n.AiringAt)
		}

		results = append(results, SearchResult{
			ID:       strconv.Itoa(item.ID),
			Title:    title,
			Image:    img,
			Score:    score,
			Type:     typ,
			EpsCount: epsCount,
			Status:   status,
			NextEp:   nextEp,
			NextTime: nextTime,
			AiringAt: airingAtVal,
		})
	}

	return results, nil
}

func (a *App) GetAnimeDetails(id string) (*Anime, error) {
	anilistID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid anime ID: %s", id)
	}

	type studioNode struct {
		Name string `json:"name"`
	}
	type mediaItem struct {
		ID             int    `json:"id"`
		Title          struct {
			Romaji  *string `json:"romaji"`
			English *string `json:"english"`
		} `json:"title"`
		CoverImage struct {
			Large *string `json:"large"`
		} `json:"coverImage"`
		AverageScore *int     `json:"averageScore"`
		Genres      []string `json:"genres"`
		Status      *string  `json:"status"`
		Episodes    *int     `json:"episodes"`
		Description *string  `json:"description"`
		StartDate   struct {
			Year  *int `json:"year"`
			Month *int `json:"month"`
			Day   *int `json:"day"`
		} `json:"startDate"`
		EndDate struct {
			Year  *int `json:"year"`
			Month *int `json:"month"`
			Day   *int `json:"day"`
		} `json:"endDate"`
		Format   *string `json:"format"`
		Duration *int    `json:"duration"`
		Source   *string `json:"source"`
		Studios  struct {
			Nodes []studioNode `json:"nodes"`
		} `json:"studios"`
		Relations struct {
			Edges []struct {
				RelationType *string `json:"relationType"`
				Node         struct {
					ID       int    `json:"id"`
					Title    struct {
						Romaji  *string `json:"romaji"`
						English *string `json:"english"`
					} `json:"title"`
					Format  *string `json:"format"`
					Episodes *int   `json:"episodes"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"relations"`
	}

	var resp struct {
		Data struct {
			Media mediaItem `json:"Media"`
		} `json:"data"`
	}

	err = a.anilistQuery(animeDetailQuery, map[string]interface{}{
		"id": anilistID,
	}, &resp)
	if err != nil {
		return nil, err
	}

	item := resp.Data.Media

	title := anilistTitle(item.Title)
	img := anilistImage(item.CoverImage)

	score := ""
	if item.AverageScore != nil {
		score = fmt.Sprintf("%.1f", float64(*item.AverageScore)/10.0)
	}
	episodes := "?"
	if item.Episodes != nil {
		episodes = strconv.Itoa(*item.Episodes)
	}
	status := ""
	if item.Status != nil {
		status = *item.Status
	}
	typ := ""
	if item.Format != nil {
		typ = *item.Format
	}
	duration := ""
	if item.Duration != nil {
		duration = fmt.Sprintf("%d min per ep", *item.Duration)
	}
	src := ""
	if item.Source != nil {
		src = *item.Source
	}
	rating := ""
	switch status {
	case "RELEASING":
		rating = "Currently Airing"
	case "FINISHED":
		rating = "Finished Airing"
	case "NOT_YET_RELEASED":
		rating = "Not Yet Aired"
	case "HIATUS":
		rating = "On Hiatus"
	default:
		rating = status
	}

	studioNames := make([]string, 0, len(item.Studios.Nodes))
	for _, s := range item.Studios.Nodes {
		studioNames = append(studioNames, s.Name)
	}

	aired := ""
	if item.StartDate.Year != nil {
		aired = fmt.Sprintf("%d", *item.StartDate.Year)
		if item.StartDate.Month != nil {
			aired += fmt.Sprintf("-%02d", *item.StartDate.Month)
		}
		if item.StartDate.Day != nil {
			aired += fmt.Sprintf("-%02d", *item.StartDate.Day)
		}
		if item.EndDate.Year != nil {
			aired += " to " + fmt.Sprintf("%d", *item.EndDate.Year)
			if item.EndDate.Month != nil {
				aired += fmt.Sprintf("-%02d", *item.EndDate.Month)
			}
		}
	}

	synopsis := ""
	if item.Description != nil {
		synopsis = *item.Description
	}

	anime := &Anime{
		ID:       item.ID,
		Title:    title,
		Image:    img,
		Score:    score,
		Genres:   item.Genres,
		Status:   status,
		Episodes: episodes,
		Synopsis: synopsis,
		Aired:    aired,
		Studios:  strings.Join(studioNames, ", "),
		Type:     typ,
		Duration: duration,
		Rating:   rating,
		Source:   src,
	}

	epList := make([]Episode, 0)
	totalEps := 0
	if item.Episodes != nil && *item.Episodes > 0 {
		totalEps = *item.Episodes
	}

	isAiring := status == "RELEASING"
	if isAiring || totalEps == 0 {
		ahCount := a.getAnimeHeavenEpCount(title)
		if ahCount > 0 {
			if isAiring && totalEps > 0 && ahCount < totalEps {
				totalEps = ahCount
			} else if totalEps == 0 {
				totalEps = ahCount
			}
			episodes = strconv.Itoa(totalEps)
			anime.Episodes = episodes
		}
	}

	if totalEps == 0 {
		anime.EpisodeList = epList
		return anime, nil
	}

	for i := 1; i <= totalEps; i++ {
		epList = append(epList, Episode{
			ID:     fmt.Sprintf("%d-%d", item.ID, i),
			Number: strconv.Itoa(i),
			Title:  fmt.Sprintf("Episode %d", i),
		})
	}
	anime.EpisodeList = epList

	return anime, nil
}

var anikotoHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	},
}

func anikotoRequest(method, urlStr string) (*http.Response, error) {
	req, err := http.NewRequest(method, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/html, */*")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", "https://anikototv.to/")
	return anikotoHTTPClient.Do(req)
}

func (a *App) searchAnikoto(query string) (string, error) {
	resp, err := anikotoRequest("GET", fmt.Sprintf("https://anikototv.to/filter?keyword=%s", url.QueryEscape(query)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	re := regexp.MustCompile(`data-tip="(\d+)"`)
	m := re.FindStringSubmatch(html)
	if len(m) < 2 {
		return "", fmt.Errorf("no anime found")
	}
	return m[1], nil
}

func (a *App) getAnikotoVideoURLs(animeID string, epNumber string) ([]StreamSource, error) {
	// Step 1: Get episode list using numeric ID
	epResp, err := anikotoRequest("GET", fmt.Sprintf("https://anikototv.to/ajax/episode/list/%s", animeID))
	if err != nil {
		return nil, fmt.Errorf("episode list failed: %w", err)
	}
	defer epResp.Body.Close()

	var epListResp struct {
		Status int `json:"status"`
		Result struct {
			HTML string `json:"html"`
		} `json:"result"`
	}
	if err := json.NewDecoder(epResp.Body).Decode(&epListResp); err != nil {
		return nil, fmt.Errorf("episode list parse failed: %w", err)
	}
	if epListResp.Status != 200 || epListResp.Result.HTML == "" {
		return nil, fmt.Errorf("episode list empty")
	}

	// Step 2: Find the episode with matching data-num
	epHTML := epListResp.Result.HTML
	epRe := regexp.MustCompile(`data-num="(\d+)"[^>]*data-slug="([^"]+)"[^>]*data-mal="(\d+)"[^>]*data-timestamp="(\d+)"`)
	epMatches := epRe.FindAllStringSubmatch(epHTML, -1)

	var epSlug, epMAL, epTimestamp string
	for _, m := range epMatches {
		num := m[1]
		if num == epNumber || strings.TrimLeft(num, "0") == epNumber {
			epSlug = m[2]
			epMAL = m[3]
			epTimestamp = m[4]
			break
		}
	}
	if epMAL == "" {
		return nil, fmt.Errorf("episode %s not found", epNumber)
	}

	log.Printf("[AniKoto] Episode %s: mal=%s slug=%s ts=%s", epNumber, epMAL, epSlug, epTimestamp)

	var sources []StreamSource

	// Step 3: Get mapper API response
	mapperURL := fmt.Sprintf("https://mapper.nekostream.site/api/mal/%s/%s/%s", epMAL, epSlug, epTimestamp)
	mResp, err := anikotoRequest("GET", mapperURL)
	if err != nil {
		return nil, fmt.Errorf("mapper API failed: %w", err)
	}
	defer mResp.Body.Close()

	var mapperResp map[string]interface{}
	if err := json.NewDecoder(mResp.Body).Decode(&mapperResp); err != nil {
		return nil, fmt.Errorf("mapper parse failed: %w", err)
	}

	// Step 4: For each source, decode the encrypted URL via ajax/server endpoint
	for sourceName, sourceData := range mapperResp {
		if sourceName == "status" {
			continue
		}
		sd, ok := sourceData.(map[string]interface{})
		if !ok {
			continue
		}

		// Sub
		if sub, ok := sd["sub"].(map[string]interface{}); ok {
			if encURL, ok := sub["url"].(string); ok && encURL != "" {
				if decoded, err := a.resolveAnikotoURL(encURL); err == nil && decoded != "" {
					sources = append(sources, StreamSource{
						Server: fmt.Sprintf("AniKoto %s Sub", sourceName),
						Type:   "video",
						Links:  []StreamLink{{URL: decoded, Quality: "sub"}},
					})
				} else {
					log.Printf("[AniKoto] Failed to decode %s sub: %v", sourceName, err)
				}
			}
		}

		// Dub
		if dub, ok := sd["dub"].(map[string]interface{}); ok {
			if encURL, ok := dub["url"].(string); ok && encURL != "" {
				if decoded, err := a.resolveAnikotoURL(encURL); err == nil && decoded != "" {
					sources = append(sources, StreamSource{
						Server: fmt.Sprintf("AniKoto %s Dub", sourceName),
						Type:   "video",
						Links:  []StreamLink{{URL: decoded, Quality: "dub"}},
					})
				} else {
					log.Printf("[AniKoto] Failed to decode %s dub: %v", sourceName, err)
				}
			}
		}
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("anikoto: no video URL found")
	}
	return sources, nil
}

// resolveAnikotoURL takes the encrypted URL from mapper, passes it through
// the AniKoto ajax/server endpoint, then decodes the base64 stream URL.
func (a *App) resolveAnikotoURL(encURL string) (string, error) {
	// Step A: Call ajax/server?get={encURL} to get the wrapper URL
	vResp, err := anikotoRequest("GET", fmt.Sprintf("https://anikototv.to/ajax/server?get=%s", encURL))
	if err != nil {
		return "", fmt.Errorf("server get failed: %w", err)
	}
	defer vResp.Body.Close()

	var vRespData struct {
		Status int `json:"status"`
		Result struct {
			URL string `json:"url"`
		} `json:"result"`
	}
	if err := json.NewDecoder(vResp.Body).Decode(&vRespData); err != nil {
		return "", fmt.Errorf("server response parse failed: %w", err)
	}
	if vRespData.Result.URL == "" {
		return "", fmt.Errorf("empty URL from server")
	}

	wrapperURL := vRespData.Result.URL
	log.Printf("[AniKoto] Wrapper URL: %s", wrapperURL[:min(80, len(wrapperURL))])

	// Step B: Decode base64 between the two # markers
	// Format: https://example.com/player/plyr.php#base64data#
	if idx1 := strings.Index(wrapperURL, "#"); idx1 >= 0 {
		if idx2 := strings.Index(wrapperURL[idx1+1:], "#"); idx2 >= 0 {
			b64Part := wrapperURL[idx1+1 : idx1+1+idx2]
			if decoded, err := base64.StdEncoding.DecodeString(b64Part); err == nil {
				streamURL := string(decoded)
				if strings.Contains(streamURL, "http") {
					log.Printf("[AniKoto] Decoded stream: %s", streamURL[:min(80, len(streamURL))])
					return streamURL, nil
				}
			}
		}
		// Fallback: try everything after first #
		b64Part := wrapperURL[idx1+1:]
		b64Part = strings.TrimRight(b64Part, "#")
		if decoded, err := base64.StdEncoding.DecodeString(b64Part); err == nil {
			streamURL := string(decoded)
			if strings.Contains(streamURL, "http") {
				log.Printf("[AniKoto] Decoded stream: %s", streamURL[:min(80, len(streamURL))])
				return streamURL, nil
			}
		}
	}

	// If no # or decode failed, try the URL directly
	if strings.Contains(wrapperURL, "http") {
		return wrapperURL, nil
	}

	return "", fmt.Errorf("could not decode URL")
}

func (a *App) GetStreamURL(episodeID string, animeTitle string) ([]StreamSource, error) {
	parts := strings.SplitN(episodeID, "-", 2)
	epNumber := ""
	anilistID := ""
	if len(parts) == 2 {
		anilistID = parts[0]
		epNumber = parts[1]
	}

	title := strings.TrimSpace(animeTitle)
	var altTitle string
	if title == "" && anilistID != "" {
		animeDetails, err := a.GetAnimeDetails(anilistID)
		if err == nil {
			title = animeDetails.Title
			if alt, err := a.getAlternateTitle(anilistID); err == nil && alt != "" {
				altTitle = alt
			}
		}
	} else if title != "" && anilistID != "" {
		if alt, err := a.getAlternateTitle(anilistID); err == nil && alt != "" {
			altTitle = alt
		}
	}

	if title == "" {
		return []StreamSource{{
			Server: "Unavailable",
			Type:   "info",
			Links:  []StreamLink{{URL: "", Quality: "anime title could not be determined"}},
		}}, nil
	}

	titles := []string{title}
	if altTitle != "" && altTitle != title {
		titles = append(titles, altTitle)
	}

	log.Printf("[Stream] Searching for: %s ep %s (alt: %s)", title, epNumber, altTitle)

	var allSources []StreamSource

	// AnimeHeaven
	ahFound := false
	for _, t := range titles {
		log.Printf("[Stream] Searching AnimeHeaven for: %s ep %s", t, epNumber)
		ahSources, err := a.getAnimeHeavenVideoAllResults(t, epNumber)
		if err == nil && len(ahSources) > 0 {
			log.Printf("[Stream] AnimeHeaven found %d source(s)", len(ahSources))
			allSources = append(allSources, ahSources...)
			ahFound = true
			break
		}
		log.Printf("[Stream] AnimeHeaven failed for '%s': %v", t, err)
	}
	if !ahFound {
		allSources = append(allSources, StreamSource{
			Server: "AnimeHeaven",
			Type:   "info",
			Links:  []StreamLink{{URL: "", Quality: "not available"}},
		})
	}

	// AniKoto
	akFound := false
	for _, t := range titles {
		log.Printf("[Stream] Searching AniKoto for: %s ep %s", t, epNumber)
		animeID, searchErr := a.searchAnikoto(t)
		if searchErr != nil {
			log.Printf("[Stream] AniKoto search failed for '%s': %v", t, searchErr)
			continue
		}
		akSources, err := a.getAnikotoVideoURLs(animeID, epNumber)
		if err == nil && len(akSources) > 0 {
			log.Printf("[Stream] AniKoto found %d source(s)", len(akSources))
			allSources = append(allSources, akSources...)
			akFound = true
			break
		}
		log.Printf("[Stream] AniKoto failed for '%s': %v", t, err)
	}
	if !akFound {
		allSources = append(allSources, StreamSource{
			Server: "AniKoto",
			Type:   "info",
			Links:  []StreamLink{{URL: "", Quality: "not available"}},
		})
	}

	// Aniwaves
	awFound := false
	for _, t := range titles {
		log.Printf("[Stream] Searching Aniwaves for: %s ep %s", t, epNumber)
		awSources, err := a.getAniwavesVideo(t, epNumber)
		if err == nil && len(awSources) > 0 {
			log.Printf("[Stream] Aniwaves found %d source(s)", len(awSources))
			allSources = append(allSources, awSources...)
			awFound = true
			break
		}
		log.Printf("[Stream] Aniwaves failed for '%s': %v", t, err)
	}
	if !awFound {
		allSources = append(allSources, StreamSource{
			Server: "Aniwaves",
			Type:   "info",
			Links:  []StreamLink{{URL: "", Quality: "not available"}},
		})
	}

	log.Printf("[Stream] Total sources: %d", len(allSources))
	return allSources, nil
}

func (a *App) getAlternateTitle(anilistID string) (string, error) {
	anilistIDInt, err := strconv.Atoi(anilistID)
	if err != nil {
		return "", err
	}

	type titleItem struct {
		Romaji  *string `json:"romaji"`
		English *string `json:"english"`
	}
	var resp struct {
		Data struct {
			Media struct {
				Title titleItem `json:"title"`
			} `json:"Media"`
		} `json:"data"`
	}

	err = a.anilistQuery(`query ($id: Int) { Media(id: $id) { title { romaji english } } }`, map[string]interface{}{
		"id": anilistIDInt,
	}, &resp)
	if err != nil {
		return "", err
	}

	t := resp.Data.Media.Title
	if t.Romaji != nil && *t.Romaji != "" {
		return *t.Romaji, nil
	}
	if t.English != nil && *t.English != "" {
		return *t.English, nil
	}
	return "", nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (a *App) searchAnimeHeaven(query string) ([]AnimeHeavenSearchResult, error) {
	searchURL := fmt.Sprintf("https://animeheaven.me/search.php?s=%s", url.QueryEscape(query))
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []AnimeHeavenSearchResult
	html := string(body)

	re := regexp.MustCompile(`anime\.php\?([a-z0-9]+)' class='c'>([^<]+)</a>`)
	matches := re.FindAllStringSubmatch(html, -1)
	seen := map[string]bool{}
	for _, m := range matches {
		id := m[1]
		title := strings.TrimSpace(m[2])
		if !seen[id] && title != "" {
			seen[id] = true
			results = append(results, AnimeHeavenSearchResult{
				ID:    id,
				Title: title,
			})
		}
	}
	if len(results) == 0 {
		re2 := regexp.MustCompile(`anime\.php\?([a-z0-9]+)[^>]*>([^<]+)</a>`)
		matches2 := re2.FindAllStringSubmatch(html, -1)
		for _, m := range matches2 {
			id := m[1]
			title := strings.TrimSpace(m[2])
			if !seen[id] && title != "" {
				seen[id] = true
				results = append(results, AnimeHeavenSearchResult{
					ID:    id,
					Title: title,
				})
			}
		}
	}

	if len(results) > 5 {
		results = results[:5]
	}
	return results, nil
}

func (a *App) getAnimeHeavenVideoAllResults(title string, epNumber string) ([]StreamSource, error) {
	ahResults, searchErr := a.searchAnimeHeaven(title)
	if searchErr != nil {
		return nil, searchErr
	}
	if len(ahResults) == 0 {
		return nil, fmt.Errorf("no results found")
	}

	epNumbers := []string{epNumber}
	if len(epNumber) == 1 {
		epNumbers = append(epNumbers, "0"+epNumber)
	} else if len(epNumber) > 1 && epNumber[0] == '0' {
		epNumbers = append(epNumbers, strings.TrimLeft(epNumber, "0"))
	}

	for _, ah := range ahResults {
		for _, epTry := range epNumbers {
			videoURL, vidErr := a.getAnimeHeavenVideo(ah.ID, epTry)
			if vidErr == nil && videoURL != "" {
				log.Printf("[Stream] AnimeHeaven got URL from %s: %s", ah.Title, videoURL[:min(80, len(videoURL))])
				return []StreamSource{{
					Server: "AnimeHeaven",
					Type:   "video",
					Links:  []StreamLink{{URL: videoURL, Quality: "auto"}},
				}}, nil
			}
		}
	}

	return nil, fmt.Errorf("episode %s not found in %d AnimeHeaven result(s)", epNumber, len(ahResults))
}

func (a *App) getAnimeHeavenEpCount(title string) int {
	ahResults, err := a.searchAnimeHeaven(title)
	if err != nil || len(ahResults) == 0 {
		return 0
	}
	best := ahResults[0]
	epURL := fmt.Sprintf("https://animeheaven.me/anime.php?%s", best.ID)
	req, err := http.NewRequest("GET", epURL, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0
	}
	html := string(body)
	re := regexp.MustCompile(`Episodes:\s*<div[^>]*>\s*(\d+)\+?`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		n, err := strconv.Atoi(m[1])
		if err == nil && n > 0 {
			return n
		}
	}
	re2 := regexp.MustCompile(`class=' watch2 bc '>\s*(\d+)`)
	matches := re2.FindAllStringSubmatch(html, -1)
	maxEp := 0
	for _, m := range matches {
		n, err := strconv.Atoi(m[1])
		if err == nil && n > maxEp {
			maxEp = n
		}
	}
	return maxEp
}

func (a *App) getAnimeHeavenVideo(animeID string, epNumber string) (string, error) {
	animeURL := fmt.Sprintf("https://animeheaven.me/anime.php?%s", animeID)
	req, err := http.NewRequest("GET", animeURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Referer", "https://animeheaven.me/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	html := string(body)

	epPad := epNumber
	if len(epNumber) == 1 {
		epPad = "0" + epNumber
	}

	// Find episode hash from gatea() onclick near Episode NUMBER
	re := regexp.MustCompile(`gatea\("([a-f0-9]{32})"\)[^<]*<[^>]*>[^<]*<[^>]*>[^<]*<[^>]*>Episode</div><div[^>]*>\s*(\d+)\s*</div>`)
	matches := re.FindAllStringSubmatch(html, -1)

	var episodeHash string
	for _, m := range matches {
		epNum := m[2]
		if len(epNum) == 1 {
			epNum = "0" + epNum
		}
		if epNum == epPad || m[2] == epNumber {
			episodeHash = m[1]
			break
		}
	}

	if episodeHash == "" {
		for _, tryNum := range []string{epPad, epNumber} {
			re2 := regexp.MustCompile(`gatea\("([a-f0-9]{32})"\)'[^<]*<div[^>]*><div[^>]*><div[^>]*>Episode</div><div[^>]*>\s*` + tryNum + `\s*</div>`)
			if m := re2.FindStringSubmatch(html); len(m) > 1 {
				episodeHash = m[1]
				break
			}
		}
		if episodeHash == "" {
			re3 := regexp.MustCompile(`gatea\("([a-f0-9]{32})"\)`)
			if m := re3.FindStringSubmatch(html); len(m) > 1 {
				episodeHash = m[1]
			}
		}
	}

	if episodeHash == "" {
		return "", fmt.Errorf("episode %s not found on animeheaven", epNumber)
	}

	// Fetch gate.php with cookie to get video URL
	gateReq, err := http.NewRequest("GET", "https://animeheaven.me/gate.php", nil)
	if err != nil {
		return "", err
	}
	gateReq.Header.Set("Cookie", fmt.Sprintf("key=%s", episodeHash))
	gateReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	gateReq.Header.Set("Referer", animeURL)

	gateResp, err := http.DefaultClient.Do(gateReq)
	if err != nil {
		return "", err
	}
	defer gateResp.Body.Close()

	gateBody, err := io.ReadAll(gateResp.Body)
	if err != nil {
		return "", err
	}
	gateHTML := string(gateBody)

	// Extract video URL from <source src='https://...video.mp4?...'>
	srcRe := regexp.MustCompile(`src='(https?://[^']+video\.mp4\?[^']+)'`)
	srcMatches := srcRe.FindStringSubmatch(gateHTML)
	if len(srcMatches) > 1 {
		return srcMatches[1], nil
	}

	// Fallback: look for any mp4 URL
	mp4Re := regexp.MustCompile(`(https?://[^"'\s]+\.mp4[^"'\s]*)`)
	mp4Matches := mp4Re.FindStringSubmatch(gateHTML)
	if len(mp4Matches) > 1 {
		return mp4Matches[1], nil
	}

	return "", fmt.Errorf("no video URL found in gate response")
}

var aniwavesHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	},
}

func aniwavesRequest(method, urlStr string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "*/*")
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return aniwavesHTTPClient.Do(req)
}

type aniwavesSearchResult struct {
	Slug    string
	AnimeID string
	Title   string
}

func (a *App) searchAniwaves(query string) ([]aniwavesSearchResult, error) {
	resp, err := aniwavesRequest("POST", "https://aniwaves.ru/ajax/anime/search",
		strings.NewReader("keyword="+url.QueryEscape(query)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResp struct {
		Status int `json:"status"`
		Result struct {
			HTML string `json:"html"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}
	html := searchResp.Result.HTML

	re := regexp.MustCompile(`/watch/([a-z0-9][a-z0-9-]+[a-z0-9])-(\d+)`)
	titleRe := regexp.MustCompile(`class="name d-title"[^>]*>([^<]+)<`)
	matches := re.FindAllStringSubmatch(html, -1)
	titleMatches := titleRe.FindAllStringSubmatch(html, -1)

	var results []aniwavesSearchResult
	seen := map[string]bool{}
	for i, m := range matches {
		id := m[2]
		if seen[id] {
			continue
		}
		seen[id] = true
		title := ""
		if i < len(titleMatches) {
			title = strings.TrimSpace(titleMatches[i][1])
		}
		results = append(results, aniwavesSearchResult{
			Slug:    m[1],
			AnimeID: id,
			Title:   title,
		})
		if len(results) >= 5 {
			break
		}
	}
	return results, nil
}

func parseAniwavesJSON(resp *http.Response) (string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var jresp struct {
		Status int    `json:"status"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal(body, &jresp); err != nil {
		return "", err
	}
	return jresp.Result, nil
}

func (a *App) getAniwavesVideo(title string, epNumber string) ([]StreamSource, error) {
	results, err := a.searchAniwaves(title)
	if err != nil {
		return nil, fmt.Errorf("aniwaves search failed: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("aniwaves: no results for %s", title)
	}

	epNumbers := []string{epNumber}
	if len(epNumber) == 1 {
		epNumbers = append(epNumbers, "0"+epNumber)
	} else if len(epNumber) > 1 && epNumber[0] == '0' {
		epNumbers = append(epNumbers, strings.TrimLeft(epNumber, "0"))
	}

	var lastErr error
	for _, result := range results {
		sources, epErr := a.getAniwavesVideoByID(result, epNumbers)
		if epErr == nil && len(sources) > 0 {
			return sources, nil
		}
		lastErr = epErr
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("aniwaves: episode %s not found in %d result(s)", epNumber, len(results))
}

func (a *App) getAniwavesVideoByID(result aniwavesSearchResult, epNumbers []string) ([]StreamSource, error) {
	animeID := result.AnimeID
	log.Printf("[Aniwaves] Trying: %s (id=%s)", result.Title, animeID)

	epResp, err := aniwavesRequest("GET", "https://aniwaves.ru/ajax/episode/list/"+animeID, nil)
	if err != nil {
		return nil, fmt.Errorf("aniwaves episode list failed: %w", err)
	}
	defer epResp.Body.Close()
	epHTML, err := parseAniwavesJSON(epResp)
	if err != nil {
		return nil, fmt.Errorf("aniwaves: failed to parse episode JSON: %w", err)
	}

	epsVal := ""
	for _, epTry := range epNumbers {
		epRe := regexp.MustCompile(`data-ids="(\d+&eps=` + regexp.QuoteMeta(epTry) + `)"`)
		if m := epRe.FindStringSubmatch(epHTML); len(m) > 1 {
			epsVal = m[1]
			break
		}
	}
	if epsVal == "" {
		return nil, fmt.Errorf("aniwaves: episode not found")
	}
	log.Printf("[Aniwaves] Episode data-ids: %s", epsVal)

	svResp, err := aniwavesRequest("GET", "https://aniwaves.ru/ajax/server/list?servers="+epsVal, nil)
	if err != nil {
		return nil, fmt.Errorf("aniwaves server list failed: %w", err)
	}
	defer svResp.Body.Close()
	svHTML, err := parseAniwavesJSON(svResp)
	if err != nil {
		return nil, fmt.Errorf("aniwaves: failed to parse server JSON: %w", err)
	}

	linkRe := regexp.MustCompile(`data-link-id="([^"]+)"`)
	allLinkMatches := linkRe.FindAllStringSubmatch(svHTML, -1)
	if len(allLinkMatches) == 0 {
		return nil, fmt.Errorf("aniwaves: no servers found")
	}

	serverNameRe := regexp.MustCompile(`data-title="([^"]+)"`)
	allServerNames := serverNameRe.FindAllStringSubmatch(svHTML, -1)

	sources := []StreamSource{}
	seenURLs := map[string]bool{}
	var mu sync.Mutex

	type serverResult struct {
		sName string
		url   string
		quality string
	}

	var wg sync.WaitGroup
	srvResults := make([]serverResult, 0, len(allLinkMatches))

	for i, lm := range allLinkMatches {
		linkID := lm[1]
		sName := "Server"
		if i < len(allServerNames) {
			sName = strings.TrimSpace(allServerNames[i][1])
		}
		log.Printf("[Aniwaves] Server %d: %s (link-id: %s)", i+1, sName, linkID[:min(20, len(linkID))])

		wg.Add(1)
		go func(lid, sn string) {
			defer wg.Done()
			srcResp, err := aniwavesRequest("GET", "https://aniwaves.ru/ajax/sources?id="+lid, nil)
			if err != nil {
				log.Printf("[Aniwaves] Failed to get source for %s: %v", sn, err)
				return
			}
			srcBody, _ := io.ReadAll(srcResp.Body)
			srcResp.Body.Close()

			var srcJSON struct {
				Status int `json:"status"`
				Result struct {
					URL     string `json:"url"`
					Server  int    `json:"server"`
					Sources []struct {
						File  string `json:"file"`
						Label string `json:"label"`
					} `json:"sources"`
				} `json:"result"`
			}
			if err := json.Unmarshal(srcBody, &srcJSON); err != nil {
				return
			}

			if len(srcJSON.Result.Sources) > 0 {
				for _, s := range srcJSON.Result.Sources {
					if s.File != "" {
						q := s.Label
						if q == "" { q = "auto" }
						mu.Lock()
						if !seenURLs[s.File] {
							seenURLs[s.File] = true
							srvResults = append(srvResults, serverResult{sName: sn, url: s.File, quality: q})
						}
						mu.Unlock()
					}
				}
			} else if srcJSON.Result.URL != "" {
				mu.Lock()
				if !seenURLs[srcJSON.Result.URL] {
					seenURLs[srcJSON.Result.URL] = true
					srvResults = append(srvResults, serverResult{sName: sn, url: srcJSON.Result.URL, quality: "auto"})
				}
				mu.Unlock()
			}
		}(linkID, sName)
	}
	wg.Wait()

	for _, r := range srvResults {
		sources = append(sources, StreamSource{
			Server: r.sName,
			Type:   "video",
			Links:  []StreamLink{{URL: r.url, Quality: r.quality}},
		})
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("aniwaves: no video URL found")
	}

	return sources, nil
}

func tryExtractM3U8(embedURL string) string {
	req, err := http.NewRequest("GET", embedURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://aniwaves.ru/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	html := string(body)

	patterns := []string{
		`(?:file|source|src|hls)\s*[:=]\s*["']([^"']*\.m3u8[^"']*)["']`,
		`["'](https?://[^"']*\.m3u8[^"']*)["']`,
		`["']([^"']*master\.m3u8[^"']*)["']`,
		`["']([^"']*index[^"']*\.m3u8[^"']*)["']`,
		`(?:url|link)\s*[:=]\s*["'](https?://[^"']*\.m3u8[^"']*)["']`,
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		m := re.FindStringSubmatch(html)
		if len(m) > 1 {
			url := m[1]
			if !strings.HasPrefix(url, "http") {
				url = "https:" + url
			}
			return url
		}
	}

	re3 := regexp.MustCompile(`["']([^"']+/playlist[^"']*\.m3u8[^"']*)["']`)
	m3 := re3.FindStringSubmatch(html)
	if len(m3) > 1 {
		url := m3[1]
		if !strings.HasPrefix(url, "http") {
			url = "https:" + url
		}
		return url
	}

	return ""
}

func (a *App) GetRecentEpisodes() ([]TrendingAnime, error) {
	now := time.Now()
	season := strings.ToUpper(fmt.Sprintf("%s", now.Month()))
	switch season {
	case "JANUARY", "FEBRUARY", "MARCH":
		season = "WINTER"
	case "APRIL", "MAY", "JUNE":
		season = "SPRING"
	case "JULY", "AUGUST", "SEPTEMBER":
		season = "SUMMER"
	case "OCTOBER", "NOVEMBER", "DECEMBER":
		season = "FALL"
	}

	type nextAiring struct {
		Episode  int `json:"episode"`
		AiringAt int `json:"airingAt"`
	}
	type mediaItem struct {
		ID             int    `json:"id"`
		Title          struct {
			Romaji  *string `json:"romaji"`
			English *string `json:"english"`
		} `json:"title"`
		CoverImage struct {
			Large *string `json:"large"`
		} `json:"coverImage"`
		AverageScore    *int        `json:"averageScore"`
		Format          *string     `json:"format"`
		Episodes        *int        `json:"episodes"`
		Status          *string     `json:"status"`
		NextAiringEpisode *nextAiring `json:"nextAiringEpisode"`
	}

	var resp struct {
		Data struct {
			Page struct {
				Media []mediaItem `json:"media"`
			} `json:"Page"`
		} `json:"data"`
	}

	err := a.anilistQuery(seasonQuery, map[string]interface{}{
		"season":     season,
		"seasonYear": now.Year(),
		"page":       1,
		"perPage":    50,
	}, &resp)
	if err != nil {
		return nil, err
	}

	var results []TrendingAnime
	for i, item := range resp.Data.Page.Media {
		title := anilistTitle(item.Title)
		img := anilistImage(item.CoverImage)
		score := ""
		if item.AverageScore != nil {
			score = fmt.Sprintf("%.1f", float64(*item.AverageScore)/10.0)
		}
		eps := "?"
		if item.Episodes != nil {
			eps = strconv.Itoa(*item.Episodes)
		}
		typ := ""
		if item.Format != nil {
			typ = *item.Format
		}
		status := ""
		if item.Status != nil {
			status = *item.Status
		}

		rank := ""
		airingAt := int64(0)
		nextEp := 0
		if item.NextAiringEpisode != nil && item.NextAiringEpisode.Episode > 0 {
			rank = fmt.Sprintf("Ep %d", item.NextAiringEpisode.Episode)
			airingAt = int64(item.NextAiringEpisode.AiringAt)
			nextEp = item.NextAiringEpisode.Episode
		} else {
			rank = strconv.Itoa(i + 1)
		}

		isNew := false
		if airingAt > 0 {
			secondsUntilNext := airingAt - time.Now().Unix()
			if secondsUntilNext > 5*86400 && secondsUntilNext < 7*86400 {
				isNew = true
			}
		}

		results = append(results, TrendingAnime{
			ID:       strconv.Itoa(item.ID),
			Title:    title,
			Image:    img,
			Rank:     rank,
			Score:    score,
			Type:     typ,
			Eps:      eps,
			AiringAt: airingAt,
			NextEp:   nextEp,
			Status:   status,
			IsNew:    isNew,
		})
	}

	return results, nil
}

func (a *App) GetFinishedAiring() ([]TrendingAnime, error) {
	type dateInfo struct {
		Year  *int `json:"year"`
		Month *int `json:"month"`
		Day   *int `json:"day"`
	}
	type mediaItem struct {
		ID             int    `json:"id"`
		Title          struct {
			Romaji  *string `json:"romaji"`
			English *string `json:"english"`
		} `json:"title"`
		CoverImage struct {
			Large *string `json:"large"`
		} `json:"coverImage"`
		AverageScore *int     `json:"averageScore"`
		Format      *string  `json:"format"`
		Episodes    *int     `json:"episodes"`
		EndDate     *dateInfo `json:"endDate"`
	}

	var resp struct {
		Data struct {
			Page struct {
				Media []mediaItem `json:"media"`
			} `json:"Page"`
		} `json:"data"`
	}

	err := a.anilistQuery(finishedQuery, map[string]interface{}{
		"page":    1,
		"perPage": 50,
	}, &resp)
	if err != nil {
		return nil, err
	}

	var results []TrendingAnime
	for i, item := range resp.Data.Page.Media {
		title := anilistTitle(item.Title)
		img := anilistImage(item.CoverImage)
		score := ""
		if item.AverageScore != nil {
			score = fmt.Sprintf("%.1f", float64(*item.AverageScore)/10.0)
		}
		eps := "?"
		if item.Episodes != nil {
			eps = strconv.Itoa(*item.Episodes)
		}
		typ := ""
		if item.Format != nil {
			typ = *item.Format
		}

		rank := ""
		if item.EndDate != nil && item.EndDate.Year != nil {
			endStr := fmt.Sprintf("%d", *item.EndDate.Year)
			if item.EndDate.Month != nil {
				endStr += fmt.Sprintf("-%02d", *item.EndDate.Month)
			}
			if item.EndDate.Day != nil {
				endStr += fmt.Sprintf("-%02d", *item.EndDate.Day)
			}
			rank = endStr
		} else {
			rank = strconv.Itoa(i + 1)
		}

		results = append(results, TrendingAnime{
			ID:    strconv.Itoa(item.ID),
			Title: title,
			Image: img,
			Rank:  rank,
			Score: score,
			Type:  typ,
			Eps:   eps,
		})
	}

	return results, nil
}

func (a *App) GetUpcoming() ([]TrendingAnime, error) {
	type dateInfo struct {
		Year  *int `json:"year"`
		Month *int `json:"month"`
		Day   *int `json:"day"`
	}
	type nextAiring struct {
		Episode  int `json:"episode"`
		AiringAt int `json:"airingAt"`
	}
	type mediaItem struct {
		ID             int    `json:"id"`
		Title          struct {
			Romaji  *string `json:"romaji"`
			English *string `json:"english"`
		} `json:"title"`
		CoverImage struct {
			Large *string `json:"large"`
		} `json:"coverImage"`
		AverageScore    *int        `json:"averageScore"`
		Format          *string     `json:"format"`
		Episodes        *int        `json:"episodes"`
		Status          *string     `json:"status"`
		StartDate       *dateInfo   `json:"startDate"`
		NextAiringEpisode *nextAiring `json:"nextAiringEpisode"`
	}

	var resp struct {
		Data struct {
			Page struct {
				Media []mediaItem `json:"media"`
			} `json:"Page"`
		} `json:"data"`
	}

	err := a.anilistQuery(upcomingQuery, map[string]interface{}{
		"page":    1,
		"perPage": 50,
	}, &resp)
	if err != nil {
		return nil, err
	}

	var results []TrendingAnime
	for i, item := range resp.Data.Page.Media {
		title := anilistTitle(item.Title)
		img := anilistImage(item.CoverImage)
		score := ""
		if item.AverageScore != nil {
			score = fmt.Sprintf("%.1f", float64(*item.AverageScore)/10.0)
		}
		eps := "?"
		if item.Episodes != nil {
			eps = strconv.Itoa(*item.Episodes)
		}
		typ := ""
		if item.Format != nil {
			typ = *item.Format
		}
		status := ""
		if item.Status != nil {
			status = *item.Status
		}

		rank := ""
		airingAt := int64(0)
		nextEp := 0
		if item.StartDate != nil && item.StartDate.Year != nil {
			startStr := fmt.Sprintf("%d", *item.StartDate.Year)
			if item.StartDate.Month != nil {
				startStr += fmt.Sprintf("-%02d", *item.StartDate.Month)
			}
			if item.StartDate.Day != nil {
				startStr += fmt.Sprintf("-%02d", *item.StartDate.Day)
			}
			rank = startStr
		} else if item.NextAiringEpisode != nil && item.NextAiringEpisode.AiringAt > 0 {
			airTime := time.Unix(int64(item.NextAiringEpisode.AiringAt), 0)
			rank = airTime.Format("Jan 2")
			airingAt = int64(item.NextAiringEpisode.AiringAt)
			nextEp = item.NextAiringEpisode.Episode
		} else {
			rank = strconv.Itoa(i + 1)
		}

		results = append(results, TrendingAnime{
			ID:       strconv.Itoa(item.ID),
			Title:    title,
			Image:    img,
			Rank:     rank,
			Score:    score,
			Type:     typ,
			Eps:      eps,
			AiringAt: airingAt,
			NextEp:   nextEp,
			Status:   status,
		})
	}

	return results, nil
}

func (a *App) GetNewFinishedAiring() ([]TrendingAnime, error) {
	type dateInfo struct {
		Year  *int `json:"year"`
		Month *int `json:"month"`
		Day   *int `json:"day"`
	}
	type mediaItem struct {
		ID             int    `json:"id"`
		Title          struct {
			Romaji  *string `json:"romaji"`
			English *string `json:"english"`
		} `json:"title"`
		CoverImage struct {
			Large *string `json:"large"`
		} `json:"coverImage"`
		AverageScore *int      `json:"averageScore"`
		Format      *string   `json:"format"`
		Episodes    *int      `json:"episodes"`
		EndDate     *dateInfo `json:"endDate"`
	}

	var resp struct {
		Data struct {
			Page struct {
				Media []mediaItem `json:"media"`
			} `json:"Page"`
		} `json:"data"`
	}

	err := a.anilistQuery(newFinishedQuery, map[string]interface{}{
		"page":    1,
		"perPage": 50,
	}, &resp)
	if err != nil {
		return nil, err
	}

	var results []TrendingAnime
	for i, item := range resp.Data.Page.Media {
		title := anilistTitle(item.Title)
		img := anilistImage(item.CoverImage)
		score := ""
		if item.AverageScore != nil {
			score = fmt.Sprintf("%.1f", float64(*item.AverageScore)/10.0)
		}
		eps := "?"
		if item.Episodes != nil {
			eps = strconv.Itoa(*item.Episodes)
		}
		typ := ""
		if item.Format != nil {
			typ = *item.Format
		}

		rank := ""
		if item.EndDate != nil && item.EndDate.Year != nil {
			endStr := fmt.Sprintf("%d", *item.EndDate.Year)
			if item.EndDate.Month != nil {
				endStr += fmt.Sprintf("-%02d", *item.EndDate.Month)
			}
			if item.EndDate.Day != nil {
				endStr += fmt.Sprintf("-%02d", *item.EndDate.Day)
			}
			rank = endStr
		} else {
			rank = strconv.Itoa(i + 1)
		}

		results = append(results, TrendingAnime{
			ID:    strconv.Itoa(item.ID),
			Title: title,
			Image: img,
			Rank:  rank,
			Score: score,
			Type:  typ,
			Eps:   eps,
		})
	}

	return results, nil
}

func (a *App) toolsDir() string {
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		for _, depth := range []string{
			filepath.Join("tools"),
			filepath.Join("..", "tools"),
			filepath.Join(exeDir, "tools"),
		} {
			var p string
			if filepath.IsAbs(depth) {
				p = depth
			} else {
				p = filepath.Join(exeDir, depth)
			}
			if _, e := os.Stat(p); e == nil {
				return p
			}
		}
	}
	cwd, _ := os.Getwd()
	cwdTools := filepath.Join(cwd, "tools")
	if _, e := os.Stat(cwdTools); e == nil {
		return cwdTools
	}
	home, _ := os.UserHomeDir()
	d := filepath.Join(home, ".animobox", "tools")
	os.MkdirAll(d, 0755)
	return d
}

func (a *App) vlcPath() string {
	// Check custom path first
	if custom := a.GetSetting("vlc_custom_path"); custom != "" {
		if _, err := os.Stat(custom); err == nil {
			return custom
		}
	}

	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(exeDir, "tools", "vlc-3.0.21", "vlc.exe"),
			filepath.Join(exeDir, "tools", "vlc", "vlc.exe"),
			filepath.Join(exeDir, "vlc.exe"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}
	cwd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(cwd, "tools", "vlc-3.0.21", "vlc.exe"),
		filepath.Join(cwd, "tools", "vlc", "vlc.exe"),
		filepath.Join(a.toolsDir(), "vlc-3.0.21", "vlc.exe"),
		filepath.Join(a.toolsDir(), "vlc", "vlc.exe"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if p, err := exec.LookPath("vlc"); err == nil {
		return p
	}
	return "vlc"
}

func (a *App) GetCustomVLCPath() string {
	return a.GetSetting("vlc_custom_path")
}

func (a *App) SetCustomVLCPath(path string) error {
	return a.SetSetting("vlc_custom_path", path)
}

func (a *App) BrowseVLCPath() string {
	filePath, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Select VLC Player Executable",
		Filters: []wruntime.FileFilter{
			{DisplayName: "Executables (*.exe)", Pattern: "*.exe"},
		},
	})
	if err != nil || filePath == "" {
		return ""
	}
	_ = a.SetCustomVLCPath(filePath)
	return filePath
}

func (a *App) TestNotification() error {
	_ = wruntime.SendNotification(a.ctx, wruntime.NotificationOptions{
		ID:    "animobox-test",
		Title: "AnimoBox",
		Body:  "Notifications are working!",
	})
	return nil
}

func (a *App) EnsureTools() error {
	vlcBin := a.vlcPath()
	if _, err := os.Stat(vlcBin); err != nil {
		if p, err := exec.LookPath("vlc"); err == nil {
			vlcBin = p
		} else {
			return fmt.Errorf("vlc not found")
		}
	}
	return nil
}

func (a *App) InitPlayer(windowID string) error {
	vlcBin := a.vlcPath()
	if _, err := os.Stat(vlcBin); err != nil {
		return fmt.Errorf("vlc not found at %s", vlcBin)
	}
	return nil
}

func (a *App) PlayInMPV(url string) error {
	vlcBin := a.vlcPath()

	if _, err := os.Stat(vlcBin); err != nil {
		return fmt.Errorf("vlc not found at %s", vlcBin)
	}

	args := []string{}

	if strings.Contains(strings.ToLower(url), ".m3u8") {
		args = append(args,
			"--http-referrer=https://anikototv.to/",
			"--http-user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		)
	} else if strings.Contains(strings.ToLower(url), "animeheaven") {
		args = append(args,
			"--http-referrer=https://animeheaven.me/",
			"--http-user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		)
	}

	args = append(args, url)

	log.Printf("[VLC] Starting: %s", vlcBin)
	log.Printf("[VLC] URL: %s", url[:min(100, len(url))])

	cmd := exec.Command(vlcBin, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		log.Printf("[VLC] Failed to start: %v", err)
		return fmt.Errorf("failed to start vlc: %w", err)
	}

	log.Printf("[VLC] Process started with PID %d", cmd.Process.Pid)

	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("[VLC] Process exited with error: %v", err)
		} else {
			log.Printf("[VLC] Process exited normally")
		}
	}()

	return nil
}

func (a *App) OpenInBrowser(url string) {
	var cmd *exec.Cmd
	switch sruntime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", strings.ReplaceAll(url, "&", "^&"))
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

func (a *App) LaunchVLC() error {
	vlcBin := a.vlcPath()
	if _, err := os.Stat(vlcBin); err != nil {
		return fmt.Errorf("vlc not found at %s", vlcBin)
	}
	log.Printf("[VLC] Launching: %s", vlcBin)
	cmd := exec.Command(vlcBin)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}

func (a *App) stopMPV() {
}

func (a *App) MPVStop() error {
	return nil
}

func (a *App) AddToLibrary(anime LibraryAnime) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, err := a.db.Exec(`
		INSERT OR REPLACE INTO library (anime_id, title, image, status, score, episodes_watch, total_episodes, last_known_episodes, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
	`, anime.AnimeID, anime.Title, anime.Image, anime.Status, anime.Score, anime.EpisodesWatch, anime.TotalEpisodes, anime.LastKnownEpisodes)
	return err
}

func (a *App) GetLibrary() ([]LibraryAnime, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := a.db.Query("SELECT id, anime_id, title, image, status, score, episodes_watch, total_episodes, last_known_episodes, updated_at FROM library ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var library []LibraryAnime
	for rows.Next() {
		var item LibraryAnime
		err := rows.Scan(&item.ID, &item.AnimeID, &item.Title, &item.Image, &item.Status, &item.Score, &item.EpisodesWatch, &item.TotalEpisodes, &item.LastKnownEpisodes, &item.UpdatedAt)
		if err != nil {
			continue
		}
		library = append(library, item)
	}
	return library, nil
}

func (a *App) RemoveFromLibrary(animeID string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := a.db.Exec("DELETE FROM library WHERE anime_id = ?", animeID)
	return err
}

func (a *App) UpdateLibraryItem(animeID string, status string, score int, episodesWatch int) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := a.db.Exec(`
		UPDATE library SET status = ?, score = ?, episodes_watch = ?, updated_at = datetime('now')
		WHERE anime_id = ?
	`, status, score, episodesWatch, animeID)
	return err
}

func (a *App) GetLibraryItem(animeID string) (*LibraryAnime, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var item LibraryAnime
	err := a.db.QueryRow("SELECT id, anime_id, title, image, status, score, episodes_watch, total_episodes, last_known_episodes, updated_at FROM library WHERE anime_id = ?", animeID).
		Scan(&item.ID, &item.AnimeID, &item.Title, &item.Image, &item.Status, &item.Score, &item.EpisodesWatch, &item.TotalEpisodes, &item.LastKnownEpisodes, &item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (a *App) ExportLibrary() (string, error) {
	library, err := a.GetLibrary()
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(library, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) SaveLibraryToFile() error {
	filePath, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title:           "Export Library",
		DefaultFilename: "animobox-library.json",
		Filters: []wruntime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil || filePath == "" {
		return err
	}
	data, err := a.ExportLibrary()
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, []byte(data), 0644)
}

func (a *App) ImportLibraryFromFile() (int, error) {
	filePath, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Import Library",
		Filters: []wruntime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil || filePath == "" {
		return 0, err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, err
	}
	return a.ImportLibrary(string(data))
}

func (a *App) ImportLibrary(jsonData string) (int, error) {
	if a.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	var items []LibraryAnime
	if err := json.Unmarshal([]byte(jsonData), &items); err != nil {
		return 0, fmt.Errorf("invalid JSON: %w", err)
	}
	count := 0
	for _, item := range items {
		_, err := a.db.Exec(`
			INSERT OR REPLACE INTO library (anime_id, title, image, status, score, episodes_watch, total_episodes, last_known_episodes, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, COALESCE(NULLIF(?, ''), datetime('now')))
		`, item.AnimeID, item.Title, item.Image, item.Status, item.Score, item.EpisodesWatch, item.TotalEpisodes, item.LastKnownEpisodes, item.UpdatedAt)
		if err == nil {
			count++
		}
	}
	return count, nil
}

func (a *App) AddToHistory(animeID string, title string, image string, episodeNumber string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := a.db.Exec(`
		INSERT INTO history (anime_id, title, image, episode_number, watched_at)
		VALUES (?, ?, ?, ?, datetime('now'))
	`, animeID, title, image, episodeNumber)
	if err != nil {
		return err
	}
	// Keep only last 500 history entries
	a.db.Exec("DELETE FROM history WHERE id NOT IN (SELECT id FROM history ORDER BY watched_at DESC LIMIT 500)")
	return nil
}

func (a *App) GetHistory() ([]HistoryItem, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := a.db.Query("SELECT id, anime_id, title, image, episode_number, watched_at FROM history ORDER BY watched_at DESC LIMIT 200")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []HistoryItem
	for rows.Next() {
		var item HistoryItem
		if err := rows.Scan(&item.ID, &item.AnimeID, &item.Title, &item.Image, &item.EpisodeNumber, &item.WatchedAt); err != nil {
			continue
		}
		history = append(history, item)
	}
	return history, nil
}

func (a *App) ClearHistory() error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := a.db.Exec("DELETE FROM history")
	return err
}

func (a *App) GetMALAuthURL() string {
	conf := a.getMALConfig()
	return conf.AuthCodeURL("state", oauth2.AccessTypeOffline)
}

func (a *App) CompleteMALAuth(code string) error {
	conf := a.getMALConfig()
	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}
	return a.saveMALToken(token)
}

func (a *App) getMALConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     "ananimobox_client_id",
		ClientSecret: "",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://myanimelist.net/v2/oauth2/authorize",
			TokenURL: "https://myanimelist.net/v2/oauth2/token",
		},
		RedirectURL: "http://localhost:2666/callback",
	}
}

func (a *App) saveMALToken(token *oauth2.Token) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := a.db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('mal_token', ?)`, token.AccessToken)
	if err != nil {
		return err
	}
	_, err = a.db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('mal_refresh_token', ?)`, token.RefreshToken)
	return err
}

func (a *App) GetMALStatus() string {
	if a.db == nil {
		return "not_connected"
	}
	var token string
	err := a.db.QueryRow("SELECT value FROM settings WHERE key = 'mal_token'").Scan(&token)
	if err != nil || token == "" {
		return "not_connected"
	}
	return "connected"
}

func (a *App) SyncToMAL() error {
	_, _ = wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Title:   "MAL Sync",
		Message: "MAL sync requires OAuth setup. Use Settings to connect your MAL account.",
		Type:    wruntime.InfoDialog,
	})
	return nil
}

func (a *App) ffmpegPath() string {
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	return "ffmpeg"
}

func (a *App) DownloadEpisode(url string, filename string) error {
	homeDir, _ := os.UserHomeDir()
	downloadDir := filepath.Join(homeDir, ".animobox", "downloads")
	os.MkdirAll(downloadDir, 0755)

	outputPath := filepath.Join(downloadDir, filename)

	ffmpegBin := a.ffmpegPath()
	if _, err := os.Stat(ffmpegBin); err != nil {
		return fmt.Errorf("ffmpeg not found, please wait for tools download")
	}

	cmd := exec.Command(ffmpegBin, "-i", url, "-c", "copy", "-y", outputPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Start()
}

func (a *App) GetDownloads() ([]string, error) {
	homeDir, _ := os.UserHomeDir()
	downloadDir := filepath.Join(homeDir, ".animobox", "downloads")

	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

func (a *App) startNotificationChecker() {
	if a.notifStopCh != nil {
		return
	}
	a.notifStopCh = make(chan struct{})
	go a.notificationLoop()
}

func (a *App) stopNotificationChecker() {
	if a.notifStopCh != nil {
		close(a.notifStopCh)
		a.notifStopCh = nil
	}
}

func (a *App) notificationLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-a.notifStopCh:
			return
		case <-ticker.C:
			if a.GetSetting("notifications_enabled") != "true" {
				continue
			}
			a.checkForNewEpisodes()
		}
	}
}

func (a *App) checkForNewEpisodes() {
	if a.db == nil {
		return
	}

	// Only check anime with status "watching"
	rows, err := a.db.Query(
		"SELECT anime_id, title, last_known_episodes FROM library WHERE status = 'watching' AND anime_id != ''",
	)
	if err != nil {
		log.Printf("[Notif] Failed to query library: %v", err)
		return
	}
	defer rows.Close()

	type checkItem struct {
		AnimeID          string
		Title            string
		LastKnownEpisodes int
	}
	var items []checkItem
	for rows.Next() {
		var item checkItem
		if err := rows.Scan(&item.AnimeID, &item.Title, &item.LastKnownEpisodes); err == nil {
			items = append(items, item)
		}
	}

	if len(items) == 0 {
		return
	}

	log.Printf("[Notif] Checking %d library anime for new episodes...", len(items))

	type checkResult struct {
		item       checkItem
		newEpCount int
	}

	var results []checkResult

	for _, item := range items {
		anilistID, err := strconv.Atoi(item.AnimeID)
		if err != nil {
			continue
		}

		var resp struct {
			Data struct {
				Media struct {
					Episodes *int `json:"episodes"`
				} `json:"Media"`
			} `json:"data"`
		}

		err = a.anilistQuery(`query ($id: Int) { Media(id: $id) { episodes } }`, map[string]interface{}{
			"id": anilistID,
		}, &resp)
		if err != nil {
			log.Printf("[Notif] Failed to check %s: %v", item.Title, err)
			continue
		}

		if resp.Data.Media.Episodes != nil && *resp.Data.Media.Episodes > item.LastKnownEpisodes {
			results = append(results, checkResult{item: item, newEpCount: *resp.Data.Media.Episodes})
		}

		// Respect AniList rate limit
		time.Sleep(300 * time.Millisecond)
	}

	if len(results) == 0 {
		return
	}

	// Send notifications and update DB
	for _, r := range results {
		epDiff := r.newEpCount - r.item.LastKnownEpisodes
		body := fmt.Sprintf("%d new episode(s) available!", epDiff)
		if epDiff == 1 {
			body = "1 new episode available!"
		}

		log.Printf("[Notif] Sending notification for %s: %s", r.item.Title, body)

		_ = wruntime.SendNotification(a.ctx, wruntime.NotificationOptions{
			ID:    fmt.Sprintf("animobox-%s", r.item.AnimeID),
			Title: r.item.Title,
			Body:  body,
		})

		// Update last_known_episodes
		_, err := a.db.Exec("UPDATE library SET last_known_episodes = ? WHERE anime_id = ?", r.newEpCount, r.item.AnimeID)
		if err != nil {
			log.Printf("[Notif] Failed to update last_known_episodes for %s: %v", r.item.AnimeID, err)
		}
	}
}

func (a *App) GetNotificationsEnabled() string {
	return a.GetSetting("notifications_enabled")
}

func (a *App) SetNotificationsEnabled(enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	return a.SetSetting("notifications_enabled", val)
}

func (a *App) SetSetting(key string, value string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := a.db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`, key, value)
	return err
}

func (a *App) GetSetting(key string) string {
	if a.db == nil {
		return ""
	}
	var value string
	err := a.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		return ""
	}
	return value
}

func (a *App) GetAppVersion() string {
	return "1.2.0"
}

func (a *App) GetPlatform() string {
	return sruntime.GOOS
}

func (a *App) MinimizeWindow() {
	wruntime.WindowMinimise(a.ctx)
}

func (a *App) MaximizeWindow() {
	wruntime.WindowToggleMaximise(a.ctx)
}

func (a *App) CloseWindow() {
	wruntime.Quit(a.ctx)
}

func (a *App) OpenFile() string {
	result, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Select Video File",
		Filters: []wruntime.FileFilter{
			{
				DisplayName: "Video Files",
				Pattern:     "*.mp4;*.mkv;*.avi;*.webm;*.flv;*.m3u8",
			},
		},
	})
	if err != nil {
		return ""
	}
	return result
}

func (a *App) OpenFolder() string {
	result, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Select Download Folder",
	})
	if err != nil {
		return ""
	}
	return result
}
