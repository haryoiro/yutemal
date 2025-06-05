package systems

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/haryoiro/yutemal/internal/api"
	"github.com/haryoiro/yutemal/internal/database"
	"github.com/haryoiro/yutemal/internal/structures"
)

// APISystem handles YouTube Music API interactions
type APISystem struct {
	config *structures.Config
	client *api.Client
	db     database.DB
}

// Cache configuration constants
const (
	cacheTTLPlaylistList   = 3600 // 1 hour in seconds
	cacheTTLPlaylistTracks = 1800 // 30 minutes in seconds
	cacheTTLSearch         = 900  // 15 minutes in seconds
	cacheTTLSections       = 1800 // 30 minutes in seconds
)

// NewAPISystem creates a new API system
func NewAPISystem(cfg *structures.Config, db database.DB) *APISystem {
	return &APISystem{
		config: cfg,
		db:     db,
	}
}

// InitializeFromHeaderFile initializes the API client from header file
func (as *APISystem) InitializeFromHeaderFile(headerPath string) error {
	client, err := api.NewClientFromHeaderFile(headerPath)
	if err != nil {
		return fmt.Errorf("failed to create YouTube API client: %w", err)
	}

	as.client = client
	return nil
}

// GetLibraryPlaylists fetches user library playlists
func (as *APISystem) GetLibraryPlaylists() ([]Playlist, error) {
	if as.client == nil {
		return nil, fmt.Errorf("API client not initialized")
	}

	// Check cache first
	cacheKey := "playlist_list:library"
	if as.db != nil {
		if cachedData, found := as.db.GetCache(cacheKey); found {
			var result []Playlist
			if err := json.Unmarshal([]byte(cachedData), &result); err == nil {
				return result, nil
			}
		}
	}

	// Fetch from API
	playlists, err := as.client.GetLibrary(api.MusicLibraryLandingEndpoint())
	if err != nil {
		return nil, err
	}

	var result []Playlist
	for _, p := range playlists {
		result = append(result, Playlist{
			ID:          p.BrowseID,
			Title:       p.Name,
			Description: p.Subtitle,
		})
	}

	// Cache the result
	if as.db != nil && len(result) > 0 {
		if data, err := json.Marshal(result); err == nil {
			_ = as.db.SetCache(cacheKey, "playlist_list", string(data), cacheTTLPlaylistList)
		}
	}

	return result, nil
}

// GetLikedPlaylists fetches user liked playlists
func (as *APISystem) GetLikedPlaylists() ([]Playlist, error) {
	if as.client == nil {
		return nil, fmt.Errorf("API client not initialized")
	}

	// Check cache first
	cacheKey := "playlist_list:liked"
	if as.db != nil {
		if cachedData, found := as.db.GetCache(cacheKey); found {
			var result []Playlist
			if err := json.Unmarshal([]byte(cachedData), &result); err == nil {
				return result, nil
			}
		}
	}

	// Fetch from API
	playlists, err := as.client.GetLibrary(api.MusicLikedPlaylistsEndpoint())
	if err != nil {
		return nil, err
	}

	var result []Playlist
	for _, p := range playlists {
		result = append(result, Playlist{
			ID:          p.BrowseID,
			Title:       p.Name,
			Description: p.Subtitle,
		})
	}

	// Cache the result
	if as.db != nil && len(result) > 0 {
		if data, err := json.Marshal(result); err == nil {
			_ = as.db.SetCache(cacheKey, "playlist_list", string(data), cacheTTLPlaylistList)
		}
	}

	return result, nil
}

// GetHomePlaylists fetches home page playlists
func (as *APISystem) GetHomePlaylists() ([]Playlist, error) {
	if as.client == nil {
		return nil, fmt.Errorf("API client not initialized")
	}

	// Check cache first
	cacheKey := "playlist_list:home"
	if as.db != nil {
		if cachedData, found := as.db.GetCache(cacheKey); found {
			var result []Playlist
			if err := json.Unmarshal([]byte(cachedData), &result); err == nil {
				return result, nil
			}
		}
	}

	// Fetch from API
	results, err := as.client.GetHomeEnhanced()
	if err != nil {
		return nil, err
	}

	var playlists []Playlist
	for _, p := range results.Playlists {
		playlists = append(playlists, Playlist{
			ID:          p.BrowseID,
			Title:       p.Name,
			Description: p.Subtitle,
		})
	}

	// Cache the result
	if as.db != nil && len(playlists) > 0 {
		if data, err := json.Marshal(playlists); err == nil {
			_ = as.db.SetCache(cacheKey, "playlist_list", string(data), cacheTTLPlaylistList)
		}
	}

	return playlists, nil
}

// GetPlaylistTracks fetches videos from a playlist
func (as *APISystem) GetPlaylistTracks(playlistID string) ([]structures.Track, error) {
	if as.client == nil {
		return nil, fmt.Errorf("API client not initialized")
	}

	// Check cache first
	cacheKey := fmt.Sprintf("playlist_tracks:%s", playlistID)
	if as.db != nil {
		if cachedData, found := as.db.GetCache(cacheKey); found {
			var result []structures.Track
			if err := json.Unmarshal([]byte(cachedData), &result); err == nil {
				return result, nil
			}
		}
	}

	// Fetch from API
	tracks, err := as.client.GetPlaylistByID(playlistID)
	if err != nil {
		return nil, err
	}

	var result []structures.Track
	for _, v := range tracks {
		result = append(result, structures.Track{
			TrackID:     v.TrackID,
			Title:       v.Title,
			Artists:     v.Artists,
			Thumbnail:   v.Thumbnail,
			Duration:    v.Duration,
			IsAvailable: v.IsAvailable,
			IsExplicit:  v.IsExplicit,
		})
	}

	// Cache the result
	if as.db != nil && len(result) > 0 {
		if data, err := json.Marshal(result); err == nil {
			_ = as.db.SetCache(cacheKey, "playlist_tracks", string(data), cacheTTLPlaylistTracks)
		}
	}

	return result, nil
}

// Search searches for music
func (as *APISystem) Search(query string) (*SearchResults, error) {
	if as.client == nil {
		return nil, fmt.Errorf("API client not initialized")
	}

	// Create a deterministic cache key from the query
	queryHash := sha256.Sum256([]byte(query))
	cacheKey := fmt.Sprintf("search:%x", queryHash)

	// Check cache first
	if as.db != nil {
		if cachedData, found := as.db.GetCache(cacheKey); found {
			var result SearchResults
			if err := json.Unmarshal([]byte(cachedData), &result); err == nil {
				return &result, nil
			}
		}
	}

	// Fetch from API
	results, err := as.client.Search(query)
	if err != nil {
		return nil, err
	}

	var videos []structures.Track
	for _, v := range results.Tracks {
		videos = append(videos, structures.Track{
			TrackID:     v.TrackID,
			Title:       v.Title,
			Artists:     v.Artists,
			Thumbnail:   v.Thumbnail,
			Duration:    v.Duration,
			IsAvailable: v.IsAvailable,
			IsExplicit:  v.IsExplicit,
		})
	}

	var playlists []Playlist
	for _, p := range results.Playlists {
		playlists = append(playlists, Playlist{
			ID:          p.BrowseID,
			Title:       p.Name,
			Description: p.Subtitle,
		})
	}

	searchResults := &SearchResults{
		Tracks:    videos,
		Playlists: playlists,
	}

	// Cache the result
	if as.db != nil {
		if data, err := json.Marshal(searchResults); err == nil {
			_ = as.db.SetCache(cacheKey, "search", string(data), cacheTTLSearch)
		}
	}

	return searchResults, nil
}

// Playlist represents a YouTube Music playlist
type Playlist struct {
	ID          string
	Title       string
	Description string
	Thumbnail   string
	VideoCount  int
}

// SearchResults contains search results
type SearchResults struct {
	Tracks    []structures.Track
	Playlists []Playlist
}

// GetHomeEnhanced fetches enhanced home page content with sections
func (as *APISystem) GetHomeEnhanced() ([]api.Section, error) {
	if as.client == nil {
		return nil, fmt.Errorf("API client not initialized")
	}

	// For now, we'll convert the existing GetHomeEnhanced result to sections
	results, err := as.client.GetHomeEnhanced()
	if err != nil {
		return nil, err
	}

	// Create sections from the results
	sections := []api.Section{}

	if len(results.Tracks) > 0 {
		trackSection := api.Section{
			Title:    "Recommended Tracks",
			Contents: []api.ContentItem{},
		}
		for _, track := range results.Tracks {
			t := track // Create a copy to avoid pointer issues
			trackSection.Contents = append(trackSection.Contents, api.ContentItem{
				Type:  "track",
				Track: &t,
			})
		}
		sections = append(sections, trackSection)
	}

	if len(results.Playlists) > 0 {
		playlistSection := api.Section{
			Title:    "Recommended Playlists",
			Contents: []api.ContentItem{},
		}
		for _, playlist := range results.Playlists {
			p := playlist // Create a copy to avoid pointer issues
			playlistSection.Contents = append(playlistSection.Contents, api.ContentItem{
				Type:     "playlist",
				Playlist: &p,
			})
		}
		sections = append(sections, playlistSection)
	}

	return sections, nil
}

// GetSections fetches all sections for the home page
func (as *APISystem) GetSections() ([]structures.Section, error) {
	if as.client == nil {
		return nil, fmt.Errorf("API client not initialized")
	}

	// Check cache first
	cacheKey := "home_sections"
	if as.db != nil {
		if cachedData, found := as.db.GetCache(cacheKey); found {
			var result []structures.Section
			if err := json.Unmarshal([]byte(cachedData), &result); err == nil {
				return result, nil
			}
		}
	}

	var sections []structures.Section

	// Recommended Playlists Section (first to show what's recommended)
	homePlaylists, err := as.GetHomePlaylists()
	if err == nil && len(homePlaylists) > 0 {
		section := structures.Section{
			ID:       "recommended",
			Title:    "Recommended for You",
			Type:     structures.SectionTypeRecommendedPlaylists,
			Contents: make([]structures.ContentItem, 0, len(homePlaylists)),
		}
		for _, playlist := range homePlaylists {
			p := structures.Playlist{
				ID:          playlist.ID,
				Title:       playlist.Title,
				Description: playlist.Description,
				Thumbnail:   playlist.Thumbnail,
				VideoCount:  playlist.VideoCount,
			}
			section.Contents = append(section.Contents, structures.ContentItem{
				Type:     "playlist",
				Playlist: &p,
			})
		}
		sections = append(sections, section)
	}

	// Library Playlists Section
	libraryPlaylists, err := as.GetLibraryPlaylists()
	if err != nil {
		// Log error but continue with other sections
		fmt.Printf("Error getting library playlists: %v\n", err)
	} else if len(libraryPlaylists) == 0 {
		fmt.Printf("Warning: No library playlists found\n")
	}

	if err == nil && len(libraryPlaylists) > 0 {
		fmt.Printf("Successfully loaded %d library playlists\n", len(libraryPlaylists))
		section := structures.Section{
			ID:       "library",
			Title:    "Your Library",
			Type:     structures.SectionTypeLibraryPlaylists,
			Contents: make([]structures.ContentItem, 0, len(libraryPlaylists)),
		}
		for _, playlist := range libraryPlaylists {
			p := structures.Playlist{
				ID:          playlist.ID,
				Title:       playlist.Title,
				Description: playlist.Description,
				Thumbnail:   playlist.Thumbnail,
				VideoCount:  playlist.VideoCount,
			}
			section.Contents = append(section.Contents, structures.ContentItem{
				Type:     "playlist",
				Playlist: &p,
			})
		}
		sections = append(sections, section)
	} else {
		fmt.Printf("Your Library section skipped - err: %v, playlist count: %d\n", err, len(libraryPlaylists))
	}

	// Liked Playlists Section
	likedPlaylists, err := as.GetLikedPlaylists()
	if err == nil && len(likedPlaylists) > 0 {
		section := structures.Section{
			ID:       "liked",
			Title:    "Liked Music",
			Type:     structures.SectionTypeLikedPlaylists,
			Contents: make([]structures.ContentItem, 0, len(likedPlaylists)),
		}
		for _, playlist := range likedPlaylists {
			p := structures.Playlist{
				ID:          playlist.ID,
				Title:       playlist.Title,
				Description: playlist.Description,
				Thumbnail:   playlist.Thumbnail,
				VideoCount:  playlist.VideoCount,
			}
			section.Contents = append(section.Contents, structures.ContentItem{
				Type:     "playlist",
				Playlist: &p,
			})
		}
		sections = append(sections, section)
	}

	// Trending Music Section (using home enhanced API for tracks)
	homeResults, err := as.client.GetHomeEnhanced()
	if err == nil && len(homeResults.Tracks) > 0 {
		section := structures.Section{
			ID:       "trending",
			Title:    "Trending Tracks",
			Type:     structures.SectionTypeHomeFeed,
			Contents: make([]structures.ContentItem, 0, len(homeResults.Tracks)),
		}
		for _, track := range homeResults.Tracks {
			t := structures.Track{
				TrackID:     track.TrackID,
				Title:       track.Title,
				Artists:     track.Artists,
				Thumbnail:   track.Thumbnail,
				Duration:    track.Duration,
				IsAvailable: track.IsAvailable,
				IsExplicit:  track.IsExplicit,
			}
			section.Contents = append(section.Contents, structures.ContentItem{
				Type:  "track",
				Track: &t,
			})
		}
		sections = append(sections, section)
	}

	// New Releases Section (placeholder - would need specific API endpoint)
	newReleasesSection := structures.Section{
		ID:       "new_releases",
		Title:    "New Releases",
		Type:     structures.SectionTypeHomeFeed,
		Contents: []structures.ContentItem{},
	}

	// Try to get some content for new releases by searching for recent popular songs
	popularSearches := []string{"new music 2024", "latest hits", "top songs"}
	for _, searchTerm := range popularSearches {
		searchResults, err := as.Search(searchTerm)
		if err == nil && len(searchResults.Tracks) > 0 {
			// Add first few tracks from search
			for i, track := range searchResults.Tracks {
				if i >= 5 { // Limit to 5 tracks per search
					break
				}
				t := structures.Track{
					TrackID:     track.TrackID,
					Title:       track.Title,
					Artists:     track.Artists,
					Thumbnail:   track.Thumbnail,
					Duration:    track.Duration,
					IsAvailable: track.IsAvailable,
					IsExplicit:  track.IsExplicit,
				}
				newReleasesSection.Contents = append(newReleasesSection.Contents, structures.ContentItem{
					Type:  "track",
					Track: &t,
				})
			}
			break // Only use first successful search
		}
	}

	if len(newReleasesSection.Contents) > 0 {
		sections = append(sections, newReleasesSection)
	}

	// Recent Activity Section (placeholder for now)
	recentSection := structures.Section{
		ID:       "recent",
		Title:    "Recent Activity",
		Type:     structures.SectionTypeRecentActivity,
		Contents: []structures.ContentItem{},
	}
	sections = append(sections, recentSection)

	// Cache the result
	if as.db != nil && len(sections) > 0 {
		if data, err := json.Marshal(sections); err == nil {
			_ = as.db.SetCache(cacheKey, "sections", string(data), cacheTTLSections)
		}
	}

	return sections, nil
}

// InvalidateCache invalidates cached data for a specific type
func (as *APISystem) InvalidateCache(cacheType string) error {
	if as.db == nil {
		return nil
	}
	return as.db.InvalidateCacheByType(cacheType)
}

// InvalidateAllCache invalidates all cached API data
func (as *APISystem) InvalidateAllCache() error {
	if as.db == nil {
		return nil
	}

	cacheTypes := []string{"playlist_list", "playlist_tracks", "search", "sections"}
	for _, cacheType := range cacheTypes {
		if err := as.db.InvalidateCacheByType(cacheType); err != nil {
			return err
		}
	}

	return nil
}

// RefreshCache forces a refresh of cached data by clearing cache and re-fetching
func (as *APISystem) RefreshCache() error {
	// Clear all cache
	if err := as.InvalidateAllCache(); err != nil {
		return err
	}

	// Pre-fetch commonly used data
	// This runs in the background to warm up the cache
	go func() {
		// Fetch home sections (includes multiple playlist types)
		_, _ = as.GetSections()
	}()

	return nil
}

// CleanExpiredCache removes expired cache entries
func (as *APISystem) CleanExpiredCache() error {
	if as.db == nil {
		return nil
	}
	return as.db.CleanExpiredCache()
}
