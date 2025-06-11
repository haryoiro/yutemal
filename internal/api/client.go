package api

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // YouTube API requires SHA1 for SAPISIDHASH
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	YTMDomain = "https://music.youtube.com"
)

// Client represents a YouTube Music API client.
type Client struct {
	sapisid         string
	innertubeAPIKey string
	clientVersion   string
	cookies         string
	accountID       string
	httpClient      *http.Client
}

// NewClient creates a new YouTube Music API client from headers.
func NewClient(headers map[string]string, accountID string) (*Client, error) {
	// Create HTTP client with headers
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Get cookies
	cookies, ok := headers["Cookie"]
	if !ok {
		return nil, fmt.Errorf("no Cookie header found")
	}

	// Extract SAPISID from cookies
	sapisid := extractSAPISID(cookies)
	if sapisid == "" {
		return nil, fmt.Errorf("no SAPISID found in cookies")
	}

	// Fetch YouTube Music homepage to get API key and client version
	req, err := http.NewRequestWithContext(context.Background(), "GET", YTMDomain, nil)
	if err != nil {
		return nil, err
	}

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	bodyStr := string(body)

	// Check if login is required
	if strings.Contains(bodyStr, `<base href="https://accounts.google.com/v3/signin/">`) ||
		strings.Contains(bodyStr, `<base href="https://consent.youtube.com/">`) {

		return nil, fmt.Errorf("need to login")
	}

	// Extract INNERTUBE_API_KEY
	apiKey := extractBetween(bodyStr, `INNERTUBE_API_KEY":"`, `"`)
	if apiKey == "" {
		return nil, fmt.Errorf("could not find INNERTUBE_API_KEY")
	}

	// Extract INNERTUBE_CLIENT_VERSION
	clientVersion := extractBetween(bodyStr, `INNERTUBE_CLIENT_VERSION":"`, `"`)
	if clientVersion == "" {
		return nil, fmt.Errorf("could not find INNERTUBE_CLIENT_VERSION")
	}

	return &Client{
		sapisid:         sapisid,
		innertubeAPIKey: apiKey,
		clientVersion:   clientVersion,
		cookies:         cookies,
		accountID:       accountID,
		httpClient:      httpClient,
	}, nil
}

// NewClientFromHeaderFile creates a client from a header file.
func NewClientFromHeaderFile(path string) (*Client, error) {
	headers := make(map[string]string)

	// Read header file
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse headers
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		headers[key] = value
	}

	// Check for required headers
	if _, ok := headers["Cookie"]; !ok {
		return nil, fmt.Errorf("no Cookie header found in file")
	}

	// Set default User-Agent if not present
	if _, ok := headers["User-Agent"]; !ok {
		headers["User-Agent"] = "Mozilla/5.0 (X11; Linux x86_64; rv:108.0) Gecko/20100101 Firefox/108.0"
	}

	// Check for account ID file
	accountID := ""
	accountPath := filepath.Join(filepath.Dir(path), "account_id.txt")
	accountData, accountErr := os.ReadFile(accountPath)

	if accountErr == nil {
		accountID = strings.TrimSpace(string(accountData))
	}

	return NewClient(headers, accountID)
}

// computeSAPIHash computes the SAPISIDHASH for authorization.
func (c *Client) computeSAPIHash() string {
	timestamp := time.Now().Unix()
	data := fmt.Sprintf("%d %s %s", timestamp, c.sapisid, YTMDomain)

	h := sha1.New() //nolint:gosec // YouTube API requires SHA1 for SAPISIDHASH
	h.Write([]byte(data))
	hash := fmt.Sprintf("%x", h.Sum(nil))

	return fmt.Sprintf("%d_%s", timestamp, hash)
}

// browse makes a browse API request.
func (c *Client) browse(endpoint Endpoint) (*BrowseResponse, error) {
	url := fmt.Sprintf("%s/youtubei/v1/%s?key=%s&prettyPrint=false",
		YTMDomain, endpoint.GetRoute(), c.innertubeAPIKey)

	// Build request body
	ctxData := map[string]any{
		"client": map[string]any{
			"clientName":    "WEB_REMIX",
			"clientVersion": c.clientVersion,
		},
	}

	if c.accountID != "" {
		ctxData["user"] = map[string]any{
			"onBehalfOfUser": c.accountID,
		}
	}

	body := map[string]any{
		"context":         ctxData,
		endpoint.GetKey(): endpoint.GetParam(),
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("SAPISIDHASH %s", c.computeSAPIHash()))
	req.Header.Set("X-Origin", YTMDomain)
	req.Header.Set("Cookie", c.cookies)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var browseResp BrowseResponse
	unmarshalErr := json.Unmarshal(respBody, &browseResp)

	if unmarshalErr != nil {
		return nil, unmarshalErr
	}

	return &browseResp, nil
}

// BrowseRaw makes a browse API request and returns raw response.
func (c *Client) BrowseRaw(endpoint Endpoint) (*BrowseResponse, error) {
	return c.browse(endpoint)
}

// GetLibrary fetches the user's library.
func (c *Client) GetLibrary(endpoint Endpoint) ([]PlaylistRef, error) {
	resp, err := c.browse(endpoint)
	if err != nil {
		return nil, err
	}

	return extractPlaylists(*resp), nil
}

// GetPlaylistByID fetches videos from a playlist by ID.
func (c *Client) GetPlaylistByID(playlistID string) ([]TrackRef, error) {
	resp, err := c.browse(PlaylistEndpoint(playlistID))
	if err != nil {
		return nil, err
	}

	return extractTracks(*resp), nil
}

// Search performs a search query.
func (c *Client) Search(query string) (*SearchResults, error) {
	resp, err := c.browse(SearchEndpoint(query))
	if err != nil {
		return nil, err
	}

	return &SearchResults{
		Tracks:    extractTracks(*resp),
		Playlists: extractPlaylists(*resp),
	}, nil
}

// GetHomeEnhanced fetches the home page content with enhanced extraction.
func (c *Client) GetHomeEnhanced() (*SearchResults, error) {
	resp, err := c.browse(MusicHomeEndpoint())
	if err != nil {
		return nil, err
	}

	return c.extractHomeContent(resp)
}

// extractHomeContent extracts content from home page response.
func (c *Client) extractHomeContent(resp *BrowseResponse) (*SearchResults, error) {
	// Use the navigation functions to extract content more thoroughly
	contents := navigateToContents(*resp)
	var tracks []TrackRef
	var playlists []PlaylistRef

	for _, content := range contents {
		tracksFromShelf, playlistsFromShelf := c.extractFromShelf(content)
		tracks = append(tracks, tracksFromShelf...)
		playlists = append(playlists, playlistsFromShelf...)

		playlistsFromGrid := c.extractFromGrid(content)
		playlists = append(playlists, playlistsFromGrid...)
	}

	// Also use the generic extractors as fallback
	genericTracks := extractTracks(*resp)
	genericPlaylists := extractPlaylists(*resp)

	// Merge results, avoiding duplicates
	finalTracks := c.deduplicateTracks(tracks, genericTracks)
	finalPlaylists := c.deduplicatePlaylists(playlists, genericPlaylists)

	return &SearchResults{
		Tracks:    finalTracks,
		Playlists: finalPlaylists,
	}, nil
}

// extractFromShelf extracts tracks and playlists from music shelf items.
func (c *Client) extractFromShelf(content any) ([]TrackRef, []PlaylistRef) {
	var tracks []TrackRef
	var playlists []PlaylistRef

	contentMap, ok := content.(map[string]any)
	if !ok {
		return tracks, playlists
	}

	shelfItems := extractMusicShelfItems(contentMap)
	if shelfItems == nil {
		return tracks, playlists
	}

	for _, item := range shelfItems {
		if track := extractTrackFromItem(item); track != nil {
			tracks = append(tracks, *track)
		}

		if playlist := extractPlaylistFromItem(item); playlist != nil {
			playlists = append(playlists, *playlist)
		}
	}

	return tracks, playlists
}

// extractFromGrid extracts playlists from grid items.
func (c *Client) extractFromGrid(content any) []PlaylistRef {
	var playlists []PlaylistRef

	contentMap, ok := content.(map[string]any)
	if !ok {
		return playlists
	}

	gridItems := extractGridItems(contentMap)
	if gridItems == nil {
		return playlists
	}

	for _, item := range gridItems {
		if playlist := extractPlaylistFromItem(item); playlist != nil {
			playlists = append(playlists, *playlist)
		}
	}

	return playlists
}

// deduplicateTracks removes duplicate tracks.
func (c *Client) deduplicateTracks(primary, secondary []TrackRef) []TrackRef {
	trackMap := make(map[string]TrackRef)

	for _, t := range primary {
		trackMap[t.TrackID] = t
	}

	for _, t := range secondary {
		trackMap[t.TrackID] = t
	}

	finalTracks := make([]TrackRef, 0, len(trackMap))
	for _, t := range trackMap {
		finalTracks = append(finalTracks, t)
	}

	return finalTracks
}

// deduplicatePlaylists removes duplicate playlists.
func (c *Client) deduplicatePlaylists(primary, secondary []PlaylistRef) []PlaylistRef {
	playlistMap := make(map[string]PlaylistRef)

	for _, p := range primary {
		playlistMap[p.BrowseID] = p
	}

	for _, p := range secondary {
		playlistMap[p.BrowseID] = p
	}

	finalPlaylists := make([]PlaylistRef, 0, len(playlistMap))
	for _, p := range playlistMap {
		finalPlaylists = append(finalPlaylists, p)
	}

	return finalPlaylists
}

// GetStreamingData fetches streaming information for a video/track.
func (c *Client) GetStreamingData(videoID string) (*StreamingData, error) {
	url := fmt.Sprintf("%s/youtubei/v1/player?key=%s&prettyPrint=false",
		YTMDomain, c.innertubeAPIKey)

	// Build request body
	ctxData := map[string]any{
		"client": map[string]any{
			"clientName":    "WEB_REMIX",
			"clientVersion": c.clientVersion,
		},
	}

	if c.accountID != "" {
		ctxData["user"] = map[string]any{
			"onBehalfOfUser": c.accountID,
		}
	}

	body := map[string]any{
		"context": ctxData,
		"videoId": videoID,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("SAPISIDHASH %s", c.computeSAPIHash()))
	req.Header.Set("X-Origin", YTMDomain)
	req.Header.Set("Cookie", c.cookies)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var playerResp PlayerResponse
	unmarshalErr := json.Unmarshal(respBody, &playerResp)

	if unmarshalErr != nil {
		return nil, unmarshalErr
	}

	// Check playability
	if playerResp.PlayabilityStatus.Status != "OK" {
		return nil, fmt.Errorf("video not playable: %s", playerResp.PlayabilityStatus.Reason)
	}

	return &playerResp.StreamingData, nil
}

// Helper functions

func extractSAPISID(cookies string) string {
	parts := strings.Split(cookies, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "SAPISID=") {
			return strings.TrimPrefix(part, "SAPISID=")
		}
	}

	return ""
}

func extractBetween(s, start, end string) string {
	startIdx := strings.Index(s, start)
	if startIdx == -1 {
		return ""
	}

	startIdx += len(start)
	endIdx := strings.Index(s[startIdx:], end)

	if endIdx == -1 {
		return ""
	}

	return s[startIdx : startIdx+endIdx]
}
