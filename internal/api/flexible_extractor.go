package api

import (
	"strconv"
	"strings"
)

// findTrackID searches for track ID in various possible locations.
func findTrackID(obj map[string]any) string {
	// Direct videoId field
	if trackID, ok := obj["videoId"].(string); ok {
		return trackID
	}

	// Navigation endpoint paths
	paths := [][]string{
		{"navigationEndpoint", "watchEndpoint", "videoId"},
		{"playNavigationEndpoint", "videoPlaybackUpsellEndpoint", "videoId"},
		{"playNavigationEndpoint", "watchEndpoint", "videoId"},
		{
			"overlay", "musicItemThumbnailOverlayRenderer", "content",
			"musicPlayButtonRenderer", "playNavigationEndpoint", "watchEndpoint", "videoId",
		},
	}

	for _, path := range paths {
		if trackID := getPathString(obj, path...); trackID != "" {
			return trackID
		}
	}

	return ""
}

// findTitle searches for title in various possible locations.
func findTitle(obj map[string]any) string {
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

// findArtists searches for artist information.
func findArtists(obj map[string]any) []string {
	// Try flexColumns approach first
	artists := extractArtistsFromFlexColumns(obj)
	if len(artists) > 0 {
		return artists
	}

	// Try subtitle approach
	artists = extractArtistsFromSubtitle(obj)
	if len(artists) > 0 {
		return artists
	}

	// Try simpleText subtitle
	return extractArtistsFromSimpleSubtitle(obj)
}

// extractArtistsFromFlexColumns extracts artists from flex columns structure.
func extractArtistsFromFlexColumns(obj map[string]any) []string {
	var artists []string

	flexCols, ok := obj["flexColumns"].([]any)
	if !ok || len(flexCols) <= 1 {
		return artists
	}

	col, ok := flexCols[1].(map[string]any)
	if !ok {
		return artists
	}

	renderer, ok := col["musicResponsiveListItemFlexColumnRenderer"].(map[string]any)
	if !ok {
		return artists
	}

	text, ok := renderer["text"].(map[string]any)
	if !ok {
		return artists
	}

	runs, ok := text["runs"].([]any)
	if !ok {
		return artists
	}

	for _, runItem := range runs {
		runObj, runOK := runItem.(map[string]any)
		if !runOK {
			continue
		}

		runText, runTextOK := runObj["text"].(string)
		if runTextOK && runText != " • " {
			artists = append(artists, runText)
		}
	}

	return artists
}

// extractArtistsFromSubtitle extracts artists from subtitle runs.
func extractArtistsFromSubtitle(obj map[string]any) []string {
	subtitle := getPathString(obj, "subtitle", "runs", "0", "text")
	if subtitle == "" {
		return nil
	}

	return parseArtistsFromString(subtitle)
}

// extractArtistsFromSimpleSubtitle extracts artists from simple subtitle text.
func extractArtistsFromSimpleSubtitle(obj map[string]any) []string {
	subtitle := getPathString(obj, "subtitle", "simpleText")
	if subtitle == "" {
		return nil
	}

	return parseArtistsFromString(subtitle)
}

// parseArtistsFromString parses artist info from a subtitle string.
func parseArtistsFromString(subtitle string) []string {
	parts := strings.Split(subtitle, " • ")
	if len(parts) == 0 {
		return nil
	}

	return []string{parts[0]}
}

// findDuration searches for duration information.
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

// findThumbnail searches for thumbnail URL.
func findThumbnail(obj map[string]any) string {
	// Try different thumbnail paths
	paths := [][]string{
		{"thumbnail", "musicThumbnailRenderer", "thumbnail", "thumbnails"},
		{"thumbnails"},
	}

	for _, path := range paths {
		thumbnails := getPath(obj, convertToInterface(path)...)
		if thumbnails == nil {
			continue
		}

		thumbArray, ok := thumbnails.([]any)
		if !ok || len(thumbArray) == 0 {
			continue
		}

		// Get the largest thumbnail (usually the last one)
		lastThumb, ok := thumbArray[len(thumbArray)-1].(map[string]any)
		if !ok {
			continue
		}

		url, ok := lastThumb["url"].(string)
		if ok {
			return url
		}
	}

	return ""
}

// getPathString gets a string value from a nested path.
func getPathString(data map[string]any, keys ...string) string {
	if result := getPath(data, convertToInterface(keys)...); result != nil {
		if s, ok := result.(string); ok {
			return s
		}
	}

	return ""
}

// convertToInterface converts string slice to interface slice.
func convertToInterface(strings []string) []any {
	interfaces := make([]any, len(strings))

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

// parseDurationString parses duration string like "3:45" to seconds.
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

// Enhanced playlist extraction.
func extractPlaylistFromObject(obj map[string]any) *PlaylistRef {
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

// findPlaylistBrowseID searches for playlist browse ID.
func findPlaylistBrowseID(obj map[string]any) string {
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

// findSubtitle searches for subtitle text.
func findSubtitle(obj map[string]any) string {
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
