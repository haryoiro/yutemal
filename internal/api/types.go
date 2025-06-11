package api

// TrackRef represents a YouTube Music track reference.
type TrackRef struct {
	TrackID     string   `json:"trackId"`
	Title       string   `json:"title"`
	Artists     []string `json:"artists"`
	Thumbnail   string   `json:"thumbnail,omitempty"`
	Duration    int      `json:"duration"` // in seconds
	IsAvailable bool     `json:"isAvailable"`
	IsExplicit  bool     `json:"isExplicit"`
}

type PlaylistTracksRef struct {
	Tracks   []TrackRef  `json:"tracks"`
	Playlist PlaylistRef `json:"playlist"`
}

// PlaylistRef represents a YouTube Music playlist reference.
type PlaylistRef struct {
	Name     string `json:"name"`
	Subtitle string `json:"subtitle"`
	BrowseID string `json:"browseId"`
}

// SearchResults contains search results.
type SearchResults struct {
	Tracks    []TrackRef    `json:"tracks"`
	Playlists []PlaylistRef `json:"playlists"`
}

// Endpoint represents an API endpoint.
type Endpoint interface {
	GetKey() string
	GetParam() string
	GetRoute() string
}

// Predefined endpoints.
type musicEndpoint struct {
	key   string
	param string
	route string
}

func (e musicEndpoint) GetKey() string   { return e.key }
func (e musicEndpoint) GetParam() string { return e.param }
func (e musicEndpoint) GetRoute() string { return e.route }

// MusicLikedPlaylistsEndpoint returns the liked playlists endpoint.
func MusicLikedPlaylistsEndpoint() Endpoint {
	return musicEndpoint{
		key:   "browseId",
		param: "FEmusic_liked_playlists",
		route: "browse",
	}
}

// MusicHomeEndpoint returns the home endpoint.
func MusicHomeEndpoint() Endpoint {
	return musicEndpoint{
		key:   "browseId",
		param: "FEmusic_home",
		route: "browse",
	}
}

// MusicLibraryLandingEndpoint returns the library landing endpoint.
func MusicLibraryLandingEndpoint() Endpoint {
	return musicEndpoint{
		key:   "browseId",
		param: "FEmusic_library_landing",
		route: "browse",
	}
}

// PlaylistEndpoint returns a playlist endpoint.
func PlaylistEndpoint(id string) Endpoint {
	return musicEndpoint{
		key:   "browseId",
		param: id,
		route: "browse",
	}
}

// SearchEndpoint returns a search endpoint.
func SearchEndpoint(query string) Endpoint {
	return musicEndpoint{
		key:   "query",
		param: query,
		route: "search",
	}
}

// VideoEndpoint returns a video/player endpoint.
func VideoEndpoint(videoID string) Endpoint {
	return musicEndpoint{
		key:   "videoId",
		param: videoID,
		route: "player",
	}
}

// BrowseResponse represents the raw API response.
type BrowseResponse map[string]any

// Section represents a content section on the home page.
type Section struct {
	Title    string        `json:"title"`
	Contents []ContentItem `json:"contents"`
}

// ContentItem represents an item in a section.
type ContentItem struct {
	Type     string       `json:"type"` // "track", "playlist", "album", etc.
	Track    *TrackRef    `json:"track,omitempty"`
	Playlist *PlaylistRef `json:"playlist,omitempty"`
}

// StreamingData represents streaming information from the player endpoint.
type StreamingData struct {
	VideoID         string       `json:"videoId"`
	Title           string       `json:"title"`
	LengthSeconds   string       `json:"lengthSeconds"`
	ChannelID       string       `json:"channelId"`
	IsLive          bool         `json:"isLive"`
	AdaptiveFormats []FormatInfo `json:"adaptiveFormats"`
	Formats         []FormatInfo `json:"formats"`
}

// FormatInfo represents audio/video format information.
type FormatInfo struct {
	ITag             int    `json:"itag"`
	URL              string `json:"url"`
	MimeType         string `json:"mimeType"`
	Bitrate          int    `json:"bitrate"`
	Width            int    `json:"width,omitempty"`
	Height           int    `json:"height,omitempty"`
	ContentLength    string `json:"contentLength,omitempty"`
	Quality          string `json:"quality"`
	QualityLabel     string `json:"qualityLabel,omitempty"`
	AudioQuality     string `json:"audioQuality,omitempty"`
	AudioSampleRate  string `json:"audioSampleRate,omitempty"`
	AudioChannels    int    `json:"audioChannels,omitempty"`
	ApproxDurationMs string `json:"approxDurationMs,omitempty"`
}

// PlayerResponse represents the response from the player endpoint.
type PlayerResponse struct {
	VideoDetails      VideoDetails      `json:"videoDetails"`
	StreamingData     StreamingData     `json:"streamingData"`
	PlayabilityStatus PlayabilityStatus `json:"playabilityStatus"`
}

// VideoDetails contains detailed video information.
type VideoDetails struct {
	VideoID          string    `json:"videoId"`
	Title            string    `json:"title"`
	LengthSeconds    string    `json:"lengthSeconds"`
	ChannelID        string    `json:"channelId"`
	ShortDescription string    `json:"shortDescription"`
	Thumbnail        Thumbnail `json:"thumbnail"`
	Author           string    `json:"author"`
}

// PlayabilityStatus indicates if the video is playable.
type PlayabilityStatus struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// Thumbnail represents thumbnail information.
type Thumbnail struct {
	Thumbnails []ThumbnailInfo `json:"thumbnails"`
}

// ThumbnailInfo represents a single thumbnail.
type ThumbnailInfo struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
