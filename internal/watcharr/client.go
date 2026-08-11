package watcharr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultUserAgent = "silo-plugin-sync-watcharr/0.1"

type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
	userAgent  string
}

type Option func(*Client)

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		if client != nil {
			c.httpClient = client
		}
	}
}

func New(baseURL, token string, opts ...Option) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	token = strings.TrimSpace(token)
	if baseURL == "" {
		return nil, errors.New("watcharr base_url is required")
	}
	if token == "" {
		return nil, errors.New("watcharr bearer token is required")
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid watcharr base_url %q", baseURL)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	c := &Client{baseURL: u, token: token, httpClient: http.DefaultClient, userAgent: defaultUserAgent}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

type MediaIDs struct {
	TMDB int    `json:"tmdb,omitempty"`
	IMDB string `json:"imdb,omitempty"`
	TVDB int    `json:"tvdb,omitempty"`
}

type Media struct {
	Type        string   `json:"type,omitempty"`
	IDs         MediaIDs `json:"ids,omitempty"`
	Name        string   `json:"name,omitempty"`
	ReleaseDate string   `json:"releaseDate,omitempty"`
	Watched     *Watched `json:"watched,omitempty"`
}

type Watched struct {
	ID              int              `json:"id"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
	Status          string           `json:"status"`
	Rating          float64          `json:"rating"`
	Plays           int              `json:"plays,omitempty"`
	WatchedEpisodes []WatchedEpisode `json:"watchedEpisodes,omitempty"`
	Media           *Media           `json:"media,omitempty"`
}

type WatchedEpisode struct {
	ID            int       `json:"id"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	Status        string    `json:"status"`
	Rating        int       `json:"rating"`
	SeasonNumber  int       `json:"seasonNumber"`
	EpisodeNumber int       `json:"episodeNumber"`
}

type PageResponse struct {
	TotalPages   int     `json:"totalPages"`
	TotalResults int     `json:"totalResults"`
	Page         int     `json:"page"`
	Results      []Media `json:"results"`
}

type UserInfo struct {
	Username    string `json:"username"`
	Type        int    `json:"type"`
	Permissions int    `json:"permissions"`
}

func (c *Client) GetUser(ctx context.Context) (UserInfo, error) {
	var out UserInfo
	err := c.do(ctx, http.MethodGet, "/api/user", nil, &out)
	return out, err
}

type AddWatchedRequest struct {
	TMDBID      int    `json:"tmdbId,omitempty"`
	ContentType string `json:"contentType"`
	Status      string `json:"status,omitempty"`
}

type AddEpisodeRequest struct {
	WatchedID     int    `json:"watchedId"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	Status        string `json:"status,omitempty"`
	Rating        int    `json:"rating,omitempty"`
}

type AddEpisodeResponse struct {
	WatchedEpisodes []WatchedEpisode `json:"watchedEpisodes"`
}

func (c *Client) ListWatched(ctx context.Context, mediaTypes ...string) ([]Media, error) {
	var all []Media
	for page := 1; ; page++ {
		q := url.Values{}
		q.Set("page", strconv.Itoa(page))
		q.Set("sort", "DATEADDED")
		q.Set("sortDir", "asc")
		for _, typ := range mediaTypes {
			if typ = strings.TrimSpace(typ); typ != "" {
				q.Add("type", typ)
			}
		}
		var resp PageResponse
		if err := c.do(ctx, http.MethodGet, "/api/watched?"+q.Encode(), nil, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Results...)
		if resp.TotalPages <= page || len(resp.Results) == 0 {
			break
		}
	}
	return all, nil
}

func (c *Client) AddMovie(ctx context.Context, tmdbID int) (Watched, error) {
	var out Watched
	err := c.do(ctx, http.MethodPost, "/api/watched", AddWatchedRequest{TMDBID: tmdbID, ContentType: "movie", Status: "FINISHED"}, &out)
	return out, err
}

func (c *Client) AddShow(ctx context.Context, tmdbID int) (Watched, error) {
	var out Watched
	err := c.do(ctx, http.MethodPost, "/api/watched", AddWatchedRequest{TMDBID: tmdbID, ContentType: "tv", Status: "FINISHED"}, &out)
	return out, err
}

func (c *Client) AddEpisode(ctx context.Context, watchedID, season, episode int) error {
	var out AddEpisodeResponse
	return c.do(ctx, http.MethodPost, "/api/watched/episode", AddEpisodeRequest{WatchedID: watchedID, SeasonNumber: season, EpisodeNumber: episode, Status: "FINISHED"}, &out)
}

func (c *Client) DeleteWatched(ctx context.Context, watchedID int) error {
	return c.do(ctx, http.MethodDelete, "/api/watched/"+strconv.Itoa(watchedID), nil, nil)
}

func (c *Client) DeleteEpisode(ctx context.Context, episodeID int) error {
	return c.do(ctx, http.MethodDelete, "/api/watched/episode/"+strconv.Itoa(episodeID), nil, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	endpoint := c.baseURL.ResolveReference(&url.URL{Path: strings.TrimRight(c.baseURL.Path, "/") + path})
	if strings.Contains(path, "?") {
		parts := strings.SplitN(path, "?", 2)
		endpoint = c.baseURL.ResolveReference(&url.URL{Path: strings.TrimRight(c.baseURL.Path, "/") + parts[0], RawQuery: parts[1]})
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("watcharr %s %s returned %d: %s", method, path, res.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode watcharr response: %w", err)
	}
	return nil
}
