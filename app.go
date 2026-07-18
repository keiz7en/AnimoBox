package main

import (
	"bytes"
	"context"
	"database/sql"
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
	ctx context.Context
	db  *sql.DB
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
}

type TrendingAnime struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Image string `json:"image"`
	Rank  string `json:"rank"`
	Score string `json:"score"`
	Subs  string `json:"subs"`
	Dubs  string `json:"dubs"`
	Type  string `json:"type"`
	Eps   string `json:"eps"`
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
	ID            int    `json:"id"`
	AnimeID       string `json:"animeId"`
	Title         string `json:"title"`
	Image         string `json:"image"`
	Status        string `json:"status"`
	Score         int    `json:"score"`
	EpisodesWatch int    `json:"episodesWatch"`
	TotalEpisodes string `json:"totalEpisodes"`
	UpdatedAt     string `json:"updatedAt"`
}

const anilistURL = "https://graphql.anilist.co"

var (
	anilistHTTPClient = &http.Client{Timeout: 15 * time.Second}
	anilistMu         sync.Mutex
	anilistLastReq    time.Time
)

const anilistRateLimit = 300 * time.Millisecond

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.initDB()
	wruntime.LogInfo(ctx, "AnimoBox started successfully")
}

func (a *App) shutdown(ctx context.Context) {
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
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`
	_, err = a.db.Exec(schema)
	if err != nil {
		log.Printf("Failed to create tables: %v", err)
	}
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
    media(type: ANIME, sort: POPULARITY_DESC) {
      id
      title { romaji english }
      coverImage { large color }
      averageScore
      format
      episodes
      status
      genres
    }
  }
}`

const searchQuery = `
query ($search: String, $page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    media(search: $search, type: ANIME) {
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
    media(type: ANIME, season: $season, seasonYear: $seasonYear, sort: POPULARITY_DESC) {
      id
      title { romaji english }
      coverImage { large color }
      averageScore
      format
      episodes
      status
      genres
    }
  }
}`

const finishedQuery = `
query ($page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    media(type: ANIME, status: FINISHED, sort: SCORE_DESC) {
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

const upcomingQuery = `
query ($page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    media(type: ANIME, status: RELEASING, sort: POPULARITY_DESC) {
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
		"perPage": 25,
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

func (a *App) SearchAnime(query string) ([]SearchResult, error) {
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

	var resp struct {
		Data struct {
			Page struct {
				Media []mediaItem `json:"media"`
			} `json:"Page"`
		} `json:"data"`
	}

	err := a.anilistQuery(searchQuery, map[string]interface{}{
		"search":  strings.TrimSpace(query),
		"page":    1,
		"perPage": 24,
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

	if totalEps == 0 {
		ahCount := a.getAnimeHeavenEpCount(title)
		if ahCount > 0 {
			totalEps = ahCount
			episodes = strconv.Itoa(ahCount)
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

func (a *App) GetStreamURL(episodeID string, animeTitle string) ([]StreamSource, error) {
	parts := strings.SplitN(episodeID, "-", 2)
	epNumber := ""
	if len(parts) == 2 {
		epNumber = parts[1]
	}

	sources := []StreamSource{}

	title := strings.TrimSpace(animeTitle)
	if title == "" {
		anilistID := ""
		if len(parts) == 2 {
			anilistID = parts[0]
		}
		animeDetails, err := a.GetAnimeDetails(anilistID)
		if err == nil {
			title = animeDetails.Title
		}
	}

	if title != "" {
		log.Printf("[Stream] Searching AnimeHeaven for: %s ep %s", title, epNumber)
		ahResults, searchErr := a.searchAnimeHeaven(title)
		if searchErr == nil && len(ahResults) > 0 {
			for _, ah := range ahResults {
				log.Printf("[Stream] Trying AnimeHeaven: %s (%s)", ah.Title, ah.ID)
				videoURL, vidErr := a.getAnimeHeavenVideo(ah.ID, epNumber)
				if vidErr == nil && videoURL != "" {
					log.Printf("[Stream] Got URL: %s", videoURL[:min(80, len(videoURL))])
					isDirectVideo := strings.Contains(strings.ToLower(videoURL), ".mp4")
					if isDirectVideo {
						sources = append(sources, StreamSource{
							Server: "AnimeHeaven",
							Type:   "video",
							Links:  []StreamLink{{URL: videoURL, Quality: "720p"}},
						})
					} else {
						sources = append(sources, StreamSource{
							Server: "AnimeHeaven",
							Type:   "embed",
							Links:  []StreamLink{{URL: videoURL, Quality: "auto"}},
						})
					}
					break
				}
			}
		}
	}

	if len(sources) == 0 && title != "" {
		log.Printf("[Stream] AnimeHeaven failed, trying Aniwaves.ru for: %s ep %s", title, epNumber)
		awSources, awErr := a.getAniwavesVideo(title, epNumber)
		if awErr == nil {
			sources = append(sources, awSources...)
		} else {
			log.Printf("[Stream] Aniwaves.ru also failed: %v", awErr)
		}
	}

	if len(sources) == 0 {
		sources = append(sources, StreamSource{
			Server: "Unavailable",
			Type:   "info",
			Links:  []StreamLink{{URL: "", Quality: "no source found"}},
		})
	}

	return sources, nil
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

	re := regexp.MustCompile(`anime\.php\?([a-z0-9]+)[^>]*class='c'>([^<]+)</a>\s*<span[^>]*>(\d+)</span>`)
	matches := re.FindAllStringSubmatch(html, -1)
	seen := map[string]bool{}
	for _, m := range matches {
		id := m[1]
		title := strings.TrimSpace(m[2])
		eps := strings.TrimSpace(m[3])
		if !seen[id] && title != "" {
			seen[id] = true
			results = append(results, AnimeHeavenSearchResult{
				ID:    id,
				Title: title,
				Eps:   eps,
			})
		}
	}
	if len(results) == 0 {
		re2 := regexp.MustCompile(`anime\.php\?([a-z0-9]+)[^>]*class='c'>([^<]+)</a>`)
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

func (a *App) getAnimeHeavenEpCount(title string) int {
	ahResults, err := a.searchAnimeHeaven(title)
	if err != nil || len(ahResults) == 0 {
		return 0
	}
	for _, r := range ahResults {
		if r.Eps != "" {
			n, err := strconv.Atoi(r.Eps)
			if err == nil && n > 0 {
				return n
			}
		}
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
	re := regexp.MustCompile(`gatea\("([^"]+)",\s*"(\d+)"`)
	matches := re.FindAllStringSubmatch(html, -1)
	maxEp := 0
	for _, m := range matches {
		n, err := strconv.Atoi(m[2])
		if err == nil && n > maxEp {
			maxEp = n
		}
	}
	if maxEp > 0 {
		return maxEp
	}
	re2 := regexp.MustCompile(`onclick='gate\((\d+)\)'`)
	matches2 := re2.FindAllStringSubmatch(html, -1)
	for _, m := range matches2 {
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

var aniwavesHTTPClient = &http.Client{Timeout: 15 * time.Second}

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
		if len(results) >= 3 {
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

	animeID := results[0].AnimeID
	log.Printf("[Aniwaves] Using: %s (id=%s)", results[0].Title, animeID)

	epResp, err := aniwavesRequest("GET", "https://aniwaves.ru/ajax/episode/list/"+animeID, nil)
	if err != nil {
		return nil, fmt.Errorf("aniwaves episode list failed: %w", err)
	}
	defer epResp.Body.Close()
	epHTML, err := parseAniwavesJSON(epResp)
	if err != nil {
		return nil, fmt.Errorf("aniwaves: failed to parse episode JSON: %w", err)
	}

	epPad := epNumber
	if len(epNumber) == 1 {
		epPad = "0" + epNumber
	}

	epsVal := ""
	epRe := regexp.MustCompile(`data-ids="(\d+&eps=` + epNumber + `)"`)
	if m := epRe.FindStringSubmatch(epHTML); len(m) > 1 {
		epsVal = m[1]
	}
	if epsVal == "" {
		epRe2 := regexp.MustCompile(`data-ids="(\d+&eps=` + epPad + `)"`)
		if m := epRe2.FindStringSubmatch(epHTML); len(m) > 1 {
			epsVal = m[1]
		}
	}
	if epsVal == "" {
		return nil, fmt.Errorf("aniwaves: episode %s not found", epNumber)
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
	linkMatches := linkRe.FindStringSubmatch(svHTML)
	if len(linkMatches) < 2 {
		return nil, fmt.Errorf("aniwaves: no servers found")
	}

	linkID := linkMatches[1]
	log.Printf("[Aniwaves] Using link-id: %s", linkID[:min(40, len(linkID))])

	srcResp, err := aniwavesRequest("GET", "https://aniwaves.ru/ajax/sources?id="+linkID, nil)
	if err != nil {
		return nil, fmt.Errorf("aniwaves sources failed: %w", err)
	}
	defer srcResp.Body.Close()
	srcBody, _ := io.ReadAll(srcResp.Body)

	var srcJSON struct {
		Status int `json:"status"`
		Result struct {
			URL     string `json:"url"`
			Server  int    `json:"server"`
			Sources []struct {
				File string `json:"file"`
			} `json:"sources"`
		} `json:"result"`
	}
	if err := json.Unmarshal(srcBody, &srcJSON); err != nil {
		return nil, fmt.Errorf("aniwaves: failed to parse source JSON: %w", err)
	}

	sources := []StreamSource{}

	if len(srcJSON.Result.Sources) > 0 {
		for _, s := range srcJSON.Result.Sources {
			if s.File != "" {
				isDirect := strings.Contains(strings.ToLower(s.File), ".mp4") || strings.Contains(strings.ToLower(s.File), ".m3u8")
				if isDirect {
					sources = append(sources, StreamSource{
						Server: "Aniwaves",
						Type:   "video",
						Links:  []StreamLink{{URL: s.File, Quality: "auto"}},
					})
				}
			}
		}
	}

	if len(sources) == 0 && srcJSON.Result.URL != "" {
		log.Printf("[Aniwaves] Got embed URL: %s", srcJSON.Result.URL[:min(80, len(srcJSON.Result.URL))])
		sources = append(sources, StreamSource{
			Server: "Aniwaves",
			Type:   "embed",
			Links:  []StreamLink{{URL: srcJSON.Result.URL, Quality: "auto"}},
		})
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("aniwaves: no video URL found")
	}

	return sources, nil
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

	err := a.anilistQuery(seasonQuery, map[string]interface{}{
		"season":     season,
		"seasonYear": now.Year(),
		"page":       1,
		"perPage":    25,
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

func (a *App) GetFinishedAiring() ([]TrendingAnime, error) {
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

	err := a.anilistQuery(finishedQuery, map[string]interface{}{
		"page":    1,
		"perPage": 25,
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

func (a *App) GetUpcoming() ([]TrendingAnime, error) {
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

	err := a.anilistQuery(upcomingQuery, map[string]interface{}{
		"page":    1,
		"perPage": 25,
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

	cmd := exec.Command(vlcBin, url)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start vlc: %w", err)
	}

	go func() {
		cmd.Wait()
	}()

	return nil
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
		INSERT OR REPLACE INTO library (anime_id, title, image, status, score, episodes_watch, total_episodes, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))
	`, anime.AnimeID, anime.Title, anime.Image, anime.Status, anime.Score, anime.EpisodesWatch, anime.TotalEpisodes)
	return err
}

func (a *App) GetLibrary() ([]LibraryAnime, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := a.db.Query("SELECT id, anime_id, title, image, status, score, episodes_watch, total_episodes, updated_at FROM library ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var library []LibraryAnime
	for rows.Next() {
		var item LibraryAnime
		err := rows.Scan(&item.ID, &item.AnimeID, &item.Title, &item.Image, &item.Status, &item.Score, &item.EpisodesWatch, &item.TotalEpisodes, &item.UpdatedAt)
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
	err := a.db.QueryRow("SELECT id, anime_id, title, image, status, score, episodes_watch, total_episodes, updated_at FROM library WHERE anime_id = ?", animeID).
		Scan(&item.ID, &item.AnimeID, &item.Title, &item.Image, &item.Status, &item.Score, &item.EpisodesWatch, &item.TotalEpisodes, &item.UpdatedAt)
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
			INSERT OR REPLACE INTO library (anime_id, title, image, status, score, episodes_watch, total_episodes, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, COALESCE(NULLIF(?, ''), datetime('now')))
		`, item.AnimeID, item.Title, item.Image, item.Status, item.Score, item.EpisodesWatch, item.TotalEpisodes, item.UpdatedAt)
		if err == nil {
			count++
		}
	}
	return count, nil
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
	return "1.0.0"
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

func (a *App) GetGenres() []string {
	return []string{
		"Action", "Adventure", "Comedy", "Drama", "Fantasy",
		"Horror", "Mahou Shoujo", "Mecha", "Music", "Mystery",
		"Psychological", "Romance", "Sci-Fi", "Slice of Life",
		"Sports", "Supernatural", "Thriller",
	}
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
