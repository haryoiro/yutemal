package api

import (
	"strconv"
	"strings"
)

// findTrackID searches for track ID in various possible locations
func findTrackID(obj map[string]interface{}) string {
	// Direct videoId field
	if trackID, ok := obj["videoId"].(string); ok {
		return trackID
	}

	// Navigation endpoint paths
	paths := [][]string{
		{"navigationEndpoint", "watchEndpoint", "videoId"},
		{"playNavigationEndpoint", "videoPlaybackUpsellEndpoint", "videoId"},
		{"playNavigationEndpoint", "watchEndpoint", "videoId"},
		{"overlay", "musicItemThumbnailOverlayRenderer", "content", "musicPlayButtonRenderer", "playNavigationEndpoint", "watchEndpoint", "videoId"},
	}

	for _, path := range paths {
		if trackID := getPathString(obj, path...); trackID != "" {
			return trackID
		}
	}

	return ""
}

// findTitle searches for title in various possible locations
func findTitle(obj map[string]interface{}) string {
	// Try different title paths
	paths := [][]string{
		{"title", "runs", "0", "text"},
		{"title", "simpleText"},
		{"flexColumns", "0", "musicResponsiveListItemFlexColumnRenderer", "text", "runs", "0", "text"},
	}

	for _, path := range paths {
		if title := getPathString(obj, path...); title != "" {
			return title
		}
	}

	return ""
}

// findArtists searches for artist information
func findArtists(obj map[string]interface{}) []string {
	var artists []string

	// Try flexColumns approach (common in music lists)
	if flexCols, ok := obj["flexColumns"].([]interface{}); ok && len(flexCols) > 1 {
		if col, ok := flexCols[1].(map[string]interface{}); ok {
			if renderer, ok := col["musicResponsiveListItemFlexColumnRenderer"].(map[string]interface{}); ok {
				if text, ok := renderer["text"].(map[string]interface{}); ok {
					if runs, ok := text["runs"].([]interface{}); ok {
						for _, run := range runs {
							if runObj, ok := run.(map[string]interface{}); ok {
								if runText, ok := runObj["text"].(string); ok && runText != " • " {
									artists = append(artists, runText)
								}
							}
						}
					}
				}
			}
		}
	}

	// Try subtitle approach
	if len(artists) == 0 {
		if subtitle := getPathString(obj, "subtitle", "runs", "0", "text"); subtitle != "" {
			// Parse subtitle for artist info
			parts := strings.Split(subtitle, " • ")
			if len(parts) > 0 {
				artists = append(artists, parts[0])
			}
		}
	}

	// Try simpleText subtitle
	if len(artists) == 0 {
		if subtitle := getPathString(obj, "subtitle", "simpleText"); subtitle != "" {
			parts := strings.Split(subtitle, " • ")
			if len(parts) > 0 {
				artists = append(artists, parts[0])
			}
		}
	}

	return artists
}


// findDuration searches for duration information
func findDuration(obj map[string]any) int {
	// Try different duration paths
	paths := [][]string{
		{"fixedColumns", "0", "musicResponsiveListItemFixedColumnRenderer", "text", "runs", "0", "text"},
		{"flexColumns", "2", "musicResponsiveListItemFlexColumnRenderer", "text", "runs", "0", "text"},
		{"lengthText", "simpleText"},
	}

	for _, path := range paths {
		if durationText := getPathString(obj, path...); durationText != "" {
			return parseDurationString(durationText)
		}
	}

	return 0
}

// findThumbnail searches for thumbnail URL
func findThumbnail(obj map[string]interface{}) string {
	// Try different thumbnail paths
	paths := [][]string{
		{"thumbnail", "musicThumbnailRenderer", "thumbnail", "thumbnails"},
		{"thumbnails"},
	}

	for _, path := range paths {
		if thumbnails := getPath(obj, convertToInterface(path)...); thumbnails != nil {
			if thumbArray, ok := thumbnails.([]interface{}); ok && len(thumbArray) > 0 {
				// Get the largest thumbnail (usually the last one)
				if lastThumb, ok := thumbArray[len(thumbArray)-1].(map[string]interface{}); ok {
					if url, ok := lastThumb["url"].(string); ok {
						return url
					}
				}
			}
		}
	}

	return ""
}



// getPathString gets a string value from a nested path
func getPathString(data map[string]interface{}, keys ...string) string {
	if result := getPath(data, convertToInterface(keys)...); result != nil {
		if s, ok := result.(string); ok {
			return s
		}
	}
	return ""
}

// convertToInterface converts string slice to interface slice
func convertToInterface(strings []string) []interface{} {
	interfaces := make([]interface{}, len(strings))
	for i, s := range strings {
		// Try to convert to int if it's a number
		if num, err := strconv.Atoi(s); err == nil {
			interfaces[i] = num
		} else {
			interfaces[i] = s
		}
	}
	return interfaces
}

// parseDurationString parses duration string like "3:45" to seconds
func parseDurationString(duration string) int {
	parts := strings.Split(duration, ":")
	if len(parts) == 0 {
		return 0
	}

	seconds := 0
	for i := len(parts) - 1; i >= 0; i-- {
		val, err := strconv.Atoi(parts[i])
		if err != nil {
			return 0
		}

		switch len(parts) - 1 - i {
		case 0: // seconds
			seconds += val
		case 1: // minutes
			seconds += val * 60
		case 2: // hours
			seconds += val * 3600
		}
	}

	return seconds
}

// Enhanced playlist extraction
func extractPlaylistFromObject(obj map[string]interface{}) *PlaylistRef {
	// Check if this looks like a playlist object
	browseID := findPlaylistBrowseID(obj)
	if browseID == "" {
		return nil
	}

	playlist := &PlaylistRef{
		BrowseID: browseID,
	}

	// Extract title
	if title := findTitle(obj); title != "" {
		playlist.Name = title
	}

	// Extract subtitle
	if subtitle := findSubtitle(obj); subtitle != "" {
		playlist.Subtitle = subtitle
	}

	// Only return if we have at least name and browse ID
	if playlist.Name != "" && playlist.BrowseID != "" {
		return playlist
	}

	return nil
}

// findPlaylistBrowseID searches for playlist browse ID
func findPlaylistBrowseID(obj map[string]interface{}) string {
	paths := [][]string{
		{"navigationEndpoint", "browseEndpoint", "browseId"},
		{"browseId"},
	}

	for _, path := range paths {
		if browseID := getPathString(obj, path...); browseID != "" {
			return browseID
		}
	}

	return ""
}

// findSubtitle searches for subtitle text
func findSubtitle(obj map[string]interface{}) string {
	paths := [][]string{
		{"subtitle", "runs", "0", "text"},
		{"subtitle", "simpleText"},
	}

	for _, path := range paths {
		if subtitle := getPathString(obj, path...); subtitle != "" {
			return subtitle
		}
	}

	return ""
}

// Enhanced playlist extraction with recursion
func extractPlaylistsRecursive(data interface{}) []PlaylistRef {
	var playlists []PlaylistRef

	switch v := data.(type) {
	case map[string]interface{}:
		// Check if this object is a playlist
		if playlist := extractPlaylistFromObject(v); playlist != nil {
			playlists = append(playlists, *playlist)
			return playlists // Don't recurse further if we found a playlist
		}

		// Recurse into object values
		for _, value := range v {
			playlists = append(playlists, extractPlaylistsRecursive(value)...)
		}

	case []interface{}:
		// Recurse into array elements
		for _, item := range v {
			playlists = append(playlists, extractPlaylistsRecursive(item)...)
		}
	}

	return playlists
}
