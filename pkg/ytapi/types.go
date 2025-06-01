package ytapi

// TrackRef represents a YouTube Music track reference
type TrackRef struct {
	TrackID     string   `json:"trackId"`
	Title       string   `json:"title"`
	Artists     []string `json:"artists"`
	Album       string   `json:"album,omitempty"`
	Thumbnail   string   `json:"thumbnail,omitempty"`
	Duration    int      `json:"duration"` // in seconds
	IsAvailable bool     `json:"isAvailable"`
	IsExplicit  bool     `json:"isExplicit"`
}

type PlaylistTracksRef struct {
	Tracks []TrackRef `json:"tracks"`
	Playlist PlaylistRef `json:"playlist"`
}


// PlaylistRef represents a YouTube Music playlist reference
type PlaylistRef struct {
	Name     string `json:"name"`
	Subtitle string `json:"subtitle"`
	BrowseID string `json:"browseId"`
}

// SearchResults contains search results
type SearchResults struct {
	Tracks    []TrackRef    `json:"tracks"`
	Playlists []PlaylistRef `json:"playlists"`
}

// Endpoint represents an API endpoint
type Endpoint interface {
	GetKey() string
	GetParam() string
	GetRoute() string
}

// Predefined endpoints
type musicEndpoint struct {
	key   string
	param string
	route string
}

func (e musicEndpoint) GetKey() string   { return e.key }
func (e musicEndpoint) GetParam() string { return e.param }
func (e musicEndpoint) GetRoute() string { return e.route }

// MusicLikedPlaylistsEndpoint returns the liked playlists endpoint
func MusicLikedPlaylistsEndpoint() Endpoint {
	return musicEndpoint{
		key:   "browseId",
		param: "FEmusic_liked_playlists",
		route: "browse",
	}
}

// MusicHomeEndpoint returns the home endpoint
func MusicHomeEndpoint() Endpoint {
	return musicEndpoint{
		key:   "browseId",
		param: "FEmusic_home",
		route: "browse",
	}
}

// MusicLibraryLandingEndpoint returns the library landing endpoint
func MusicLibraryLandingEndpoint() Endpoint {
	return musicEndpoint{
		key:   "browseId",
		param: "FEmusic_library_landing",
		route: "browse",
	}
}

// PlaylistEndpoint returns a playlist endpoint
func PlaylistEndpoint(id string) Endpoint {
	return musicEndpoint{
		key:   "browseId",
		param: id,
		route: "browse",
	}
}

// SearchEndpoint returns a search endpoint
func SearchEndpoint(query string) Endpoint {
	return musicEndpoint{
		key:   "query",
		param: query,
		route: "search",
	}
}

// BrowseResponse represents the raw API response
type BrowseResponse map[string]interface{}

// Continuation represents pagination info
type Continuation struct {
	Token               string `json:"continuation"`
	ClickTrackingParams string `json:"clickTrackingParams"`
}

// Section represents a content section on the home page
type Section struct {
	Title    string        `json:"title"`
	Contents []ContentItem `json:"contents"`
}

// ContentItem represents an item in a section
type ContentItem struct {
	Type     string       `json:"type"` // "track", "playlist", "album", etc.
	Track    *TrackRef    `json:"track,omitempty"`
	Playlist *PlaylistRef `json:"playlist,omitempty"`
}
