package api

import (
	"strconv"
)

// fromJSON applies a transformer function recursively to extract data from JSON
// This matches the Rust implementation's approach
func fromJSON[T any](data any, transformer func(any) *T, keyFunc func(T) string) []T {
	var results []T
	seen := make(map[string]bool)

	var crawl func(any)
	crawl = func(value any) {
		// Try to transform this value
		if result := transformer(value); result != nil {
			key := keyFunc(*result)
			if !seen[key] {
				results = append(results, *result)
				seen[key] = true
			}
			return // Don't recurse if we found something
		}

		// Recurse into the structure
		switch v := value.(type) {
		case map[string]any:
			for _, val := range v {
				crawl(val)
			}
		case BrowseResponse: // Handle the type alias
			for _, val := range v {
				crawl(val)
			}
		case []any:
			for _, item := range v {
				crawl(item)
			}
		}
	}

	crawl(data)
	return results
}

// extractPlaylists extracts playlist references from API response
func extractPlaylists(resp BrowseResponse) []PlaylistRef {
	return fromJSON(resp, extractPlaylistFromAny, func(p PlaylistRef) string { return p.BrowseID })
}

// extractTracks extracts video references from API response
func extractTracks(resp BrowseResponse) []TrackRef {
	return fromJSON(resp, extractTrackFromItem, func(v TrackRef) string { return v.TrackID })
}

// extractPlaylistFromAny tries to extract a playlist from any JSON value
func extractPlaylistFromAny(value any) *PlaylistRef {
	obj, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	// Check if this is a musicTwoRowItemRenderer or similar playlist container
	var playlistObj map[string]any

	// Look for musicTwoRowItemRenderer first (library playlists)
	if renderer, ok := obj["musicTwoRowItemRenderer"].(map[string]any); ok {
		playlistObj = renderer
	} else if renderer, ok := obj["musicResponsiveListItemRenderer"].(map[string]any); ok {
		playlistObj = renderer
	} else {
		// If it's already a playlist object, use it directly
		playlistObj = obj
	}

	// Convert to map[string]interface{} for flexible_extractor functions
	objInterface := make(map[string]interface{})
	for k, v := range playlistObj {
		objInterface[k] = v
	}

	// Use flexible_extractor function
	return extractPlaylistFromObject(objInterface)
}

// extractTrackFromItem tries to extract a video from any JSON value
func extractTrackFromItem(value any) *TrackRef {
	obj, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	// Convert map[string]any to map[string]interface{} for flexible_extractor functions
	objInterface := make(map[string]interface{})
	for k, v := range obj {
		objInterface[k] = v
	}

	// Use flexible_extractor functions for better extraction
	trackID := findTrackID(objInterface)
	if trackID == "" {
		return nil
	}

	title := findTitle(objInterface)
	if title == "" {
		return nil
	}

	artists := findArtists(objInterface)
	album := findAlbum(objInterface)
	duration := findDuration(objInterface)
	thumbnail := findThumbnail(objInterface)

	return &TrackRef{
		TrackID:     trackID,
		Title:       title,
		Artists:     artists,
		Album:       album,
		Duration:    duration,
		Thumbnail:   thumbnail,
		IsAvailable: true,
	}
}

// getText extracts text from a nested path in JSON
func getText(obj any, path []string) string {
	current := obj

	for _, key := range path {
		switch curr := current.(type) {
		case map[string]any:
			current = curr[key]
		case []any:
			// Try to parse key as index
			if idx, err := strconv.Atoi(key); err == nil && idx < len(curr) {
				current = curr[idx]
			} else {
				return ""
			}
		default:
			return ""
		}

		if current == nil {
			return ""
		}
	}

	if str, ok := current.(string); ok {
		return str
	}
	return ""
}

// Utility functions

// getPath extracts a value from nested path in JSON
func getPath(data map[string]any, keys ...any) any {
	current := any(data)

	for _, key := range keys {
		switch k := key.(type) {
		case string:
			m, ok := current.(map[string]any)
			if !ok {
				return nil
			}
			current = m[k]
		case int:
			a, ok := current.([]any)
			if !ok || k >= len(a) {
				return nil
			}
			current = a[k]
		default:
			return nil
		}
	}

	return current
}

// interfaceSliceToMapSlice converts []any to []map[string]any
func interfaceSliceToMapSlice(slice []any) []map[string]any {
	var result []map[string]any
	for _, item := range slice {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

// Helper functions to navigate the response structure

func navigateToContents(resp BrowseResponse) []map[string]any {
	var contents []map[string]any

	// Try different paths where content might be located
	if c := getContents(resp); c != nil {
		contents = append(contents, c...)
	}

	if tabs := getTabs(resp); tabs != nil {
		for _, tab := range tabs {
			if c := getTabContents(tab); c != nil {
				contents = append(contents, c...)
			}
		}
	}

	return contents
}

func getContents(data map[string]any) []map[string]any {
	if contents, ok := getPath(data, "contents").(map[string]any); ok {
		return extractContentItems(contents)
	}
	return nil
}

func getTabs(data map[string]any) []map[string]any {
	if tabs, ok := getPath(data, "contents", "singleColumnBrowseResultsRenderer", "tabs").([]any); ok {
		return interfaceSliceToMapSlice(tabs)
	}
	return nil
}

func getTabContents(tab map[string]any) []map[string]any {
	if content, ok := getPath(tab, "tabRenderer", "content").(map[string]any); ok {
		return extractContentItems(content)
	}
	return nil
}

func extractContentItems(content map[string]any) []map[string]any {
	var items []map[string]any

	// Check different content types
	if sectionList, ok := content["sectionListRenderer"].(map[string]any); ok {
		if contents, ok := sectionList["contents"].([]any); ok {
			items = append(items, interfaceSliceToMapSlice(contents)...)
		}
	}

	if musicShelf, ok := content["musicShelfRenderer"].(map[string]any); ok {
		items = append(items, musicShelf)
	}

	if musicCarousel, ok := content["musicCarouselShelfRenderer"].(map[string]any); ok {
		items = append(items, musicCarousel)
	}

	return items
}

func extractMusicShelfItems(content map[string]any) []map[string]any {
	if shelf, ok := content["musicShelfRenderer"].(map[string]any); ok {
		if contents, ok := shelf["contents"].([]any); ok {
			return interfaceSliceToMapSlice(contents)
		}
	}
	return nil
}

func extractGridItems(content map[string]any) []map[string]any {
	if grid, ok := content["gridRenderer"].(map[string]any); ok {
		if items, ok := grid["items"].([]any); ok {
			return interfaceSliceToMapSlice(items)
		}
	}
	return nil
}

func extractPlaylistPanelItems(content map[string]any) []map[string]any {
	if panel, ok := content["musicPlaylistShelfRenderer"].(map[string]any); ok {
		if contents, ok := panel["contents"].([]any); ok {
			return interfaceSliceToMapSlice(contents)
		}
	}
	return nil
}

func extractPlaylistFromItem(item map[string]any) *PlaylistRef {
	// Try different renderer types
	renderers := []string{
		"musicTwoRowItemRenderer",
		"musicResponsiveListItemRenderer",
		"playlistPanelVideoRenderer",
	}

	for _, renderer := range renderers {
		if data, ok := item[renderer].(map[string]any); ok {
			playlist := &PlaylistRef{}

			// Extract title
			if title := extractTitle(data); title != "" {
				playlist.Name = title
			}

			// Extract subtitle
			if subtitle := extractSubtitle(data); subtitle != "" {
				playlist.Subtitle = subtitle
			}

			// Extract browse ID
			if browseID := extractBrowseID(data); browseID != "" {
				playlist.BrowseID = browseID
			}

			if playlist.Name != "" && playlist.BrowseID != "" {
				return playlist
			}
		}
	}

	return nil
}

func extractTitle(data map[string]any) string {
	objInterface := make(map[string]interface{})
	for k, v := range data {
		objInterface[k] = v
	}
	return findTitle(objInterface)
}

func extractSubtitle(data map[string]any) string {
	objInterface := make(map[string]interface{})
	for k, v := range data {
		objInterface[k] = v
	}
	return findSubtitle(objInterface)
}

func extractBrowseID(data map[string]any) string {
	objInterface := make(map[string]interface{})
	for k, v := range data {
		objInterface[k] = v
	}
	return findPlaylistBrowseID(objInterface)
}
