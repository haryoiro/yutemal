package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/haryoiro/yutemal/internal/structures"
)

// マウスイベントのハンドリング.
func (m *Model) handleMouseEvent(mouse tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch mouse.Action {
	case tea.MouseActionPress:
		if mouse.Button == tea.MouseButtonLeft {
			return m.handleMouseClick(mouse.X, mouse.Y)
		}

		if mouse.Button == tea.MouseButtonWheelUp {
			return m.handleScrollUp()
		}

		if mouse.Button == tea.MouseButtonWheelDown {
			return m.handleScrollDown()
		}
	}

	return m, nil
}

// マウスクリックの処理.
func (m *Model) handleMouseClick(x, y int) (tea.Model, tea.Cmd) {
	// プレイヤー部分の高さを計算
	// プレイヤーの高さには、フレーム（上下各1）も含まれている
	playerAreaStart := m.height - m.playerHeight

	// プレイヤーエリアのクリック
	if y >= playerAreaStart {
		return m.handlePlayerClick(x, y-playerAreaStart)
	}

	// コンテンツエリアのクリック
	return m.handleContentClick(x, y)
}

// プレイヤー部分のクリック処理.
func (m *Model) handlePlayerClick(x, y int) (tea.Model, tea.Cmd) {
	// プログレスバーの位置を計算
	// フレームのマージンを考慮（左右各1文字分）
	contentX := x - 1
	if contentX < 0 {
		return m, nil
	}

	// プレイヤーUIの構造:
	// yはプレイヤーエリア内の相対位置（0から始まる）
	// 2行上が当たり判定ということは、クリック位置から2を引く必要がある
	adjustedY := y + 2
	if adjustedY == 3 && m.playerState.TotalTime > 0 { // プログレスバーの実際の位置
		// 時刻表示の幅を計算
		// フォーマット: "00:00 [プログレスバー] 00:00"
		// 時刻表示は "00:00" = 5文字、その後のスペース = 1文字、合計6文字
		timeDisplayWidth := 6
		progressBarStart := timeDisplayWidth

		// プログレスバーの幅を計算
		// playerContentWidthはパディングを含んだ幅なので、左右の時刻表示分を引く
		// 時刻表示は左右に6文字ずつ（"00:00 " と " 00:00"）
		barWidth := m.playerContentWidth - (timeDisplayWidth * 2)
		if barWidth <= 0 {
			return m, nil
		}

		// クリック位置がプログレスバー内かチェック
		if contentX >= progressBarStart && contentX < progressBarStart+barWidth {
			// クリック位置から進行度を計算
			clickPos := contentX - progressBarStart

			progress := float64(clickPos) / float64(barWidth)
			if progress < 0 {
				progress = 0
			} else if progress > 1 {
				progress = 1
			}

			// シーク位置を計算
			seekPos := time.Duration(float64(m.playerState.TotalTime) * progress)

			// シークアクションを送信
			m.systems.Player.SendAction(structures.SeekAction{Position: seekPos})
		}
	}

	return m, nil
}

// コンテンツ部分のクリック処理.
func (m *Model) handleContentClick(x, y int) (tea.Model, tea.Cmd) {
	// フレームボーダー（上部の枠線）を考慮
	// lipgloss.RoundedBorder()は上下左右に1文字分のボーダーを追加
	contentY := y - 1

	// ビューに応じて処理を分岐
	switch m.state {
	case PlaylistDetailView:
		// プレイリスト詳細ビュー
		// renderPlaylistDetailの構造:
		// - タイトル行: "🎶 PLAYLIST: xxx"
		// - ショートカット行: "[Enter/l: Play from Here] ..."
		// - 改行: "\n"
		// - リストアイテムが始まる
		// 実際には、表示位置が1つずれているため、調整が必要
		listStartY := 4
		relativeY := contentY - listStartY

		// 表示範囲内かチェック
		if relativeY >= 0 && relativeY < m.contentHeight {
			clickedIndex := m.playlistScrollOffset + relativeY

			// 最大選択可能インデックスを取得
			if clickedIndex >= 0 && clickedIndex < len(m.playlistTracks) {
				m.playlistSelectedIndex = clickedIndex
				// 即座に再生
				return m.playSelectedTrack()
			}
		}

	case SearchView:
		// 検索ビュー
		// タイトル行(1行) + Query行(1行) + 空行(1行) = 3行分のオフセット
		listStartY := 3
		relativeY := contentY - listStartY

		// 表示範囲内かチェック
		if relativeY >= 0 && relativeY < m.contentHeight {
			clickedIndex := m.scrollOffset + relativeY

			if clickedIndex >= 0 && clickedIndex < len(m.searchResults) {
				m.selectedIndex = clickedIndex
				return m.playSelectedTrack()
			}
		}

	case HomeView:
		// ホーム画面
		// セクションの内容の開始位置を計算
		// タブがない場合: セクションタイトル(1行) + ボーダー(1行) = 2行
		listStartY := 6

		if m.currentSectionIndex < len(m.sections) {
			section := m.sections[m.currentSectionIndex]
			relativeY := contentY - listStartY

			// 表示範囲内かチェック
			if relativeY >= 0 && relativeY < m.contentHeight {
				clickedIndex := m.scrollOffset + relativeY

				if clickedIndex >= 0 && clickedIndex < len(section.Contents) {
					m.selectedIndex = clickedIndex
					// Enterキーと同じ動作（選択されたコンテンツに応じて処理）
					content := section.Contents[m.selectedIndex]
					if content.Type == "playlist" && content.Playlist != nil {
						// プレイリストを開く
						playlist := content.Playlist
						// Reset playlist view state
						m.playlistTracks = []structures.Track{}
						m.playlistName = playlist.Title
						m.playlistSelectedIndex = 0
						m.playlistScrollOffset = 0
						m.state = PlaylistDetailView
						// Keep backward compatibility
						m.currentList = []structures.Track{}
						m.currentListName = playlist.Title

						return m, m.loadPlaylistTracks(playlist.ID)
					} else if content.Type == "track" && content.Track != nil {
						// トラックを再生
						track := content.Track

						m.systems.Player.SendAction(structures.CleanupAction{})
						m.systems.Player.SendAction(structures.AddTrackAction{Track: *track})
						m.systems.Player.SendAction(structures.PlayAction{})
					}
				}
			}
		}

	case PlaylistListView:
		// プレイリスト一覧ビュー
		// タイトル行(1行) = 1行分のオフセット
		listStartY := 1
		relativeY := contentY - listStartY

		// 表示範囲内かチェック
		if relativeY >= 0 && relativeY < m.contentHeight {
			clickedIndex := m.scrollOffset + relativeY

			if clickedIndex >= 0 && clickedIndex < len(m.playlists) {
				m.selectedIndex = clickedIndex
				// プレイリストを開く
				playlist := m.playlists[m.selectedIndex]
				// Reset playlist view state
				m.playlistTracks = []structures.Track{}
				m.playlistName = playlist.Title
				m.playlistSelectedIndex = 0
				m.playlistScrollOffset = 0
				m.state = PlaylistDetailView
				// Keep backward compatibility
				m.currentList = []structures.Track{}
				m.currentListName = playlist.Title

				return m, m.loadPlaylistTracks(playlist.ID)
			}
		}
	}

	return m, nil
}

// スクロールアップの処理.
func (m *Model) handleScrollUp() (tea.Model, tea.Cmd) {
	// スロットリング: 前回のスクロールから一定時間経過していない場合は無視
	now := time.Now()
	if now.Sub(m.lastScrollTime) < m.scrollCooldown {
		return m, nil
	}

	m.lastScrollTime = now

	// Queue画面がフォーカスされている場合
	if m.queueFocused && m.showQueue {
		if m.queueSelectedIndex > 0 {
			m.queueSelectedIndex--
			// Adjust scroll to keep selection visible
			if m.queueSelectedIndex < m.queueScrollOffset {
				m.queueScrollOffset = m.queueSelectedIndex
			}
		}

		return m, nil
	}

	// 通常のコンテンツスクロール
	switch m.state {
	case PlaylistDetailView:
		if m.playlistSelectedIndex > 0 {
			m.playlistSelectedIndex--
			m.adjustPlaylistScroll()
		}
	default:
		if m.selectedIndex > 0 {
			m.selectedIndex--
			m.adjustScroll()
		}
	}

	return m, nil
}

// スクロールダウンの処理.
func (m *Model) handleScrollDown() (tea.Model, tea.Cmd) {
	// スロットリング: 前回のスクロールから一定時間経過していない場合は無視
	now := time.Now()
	if now.Sub(m.lastScrollTime) < m.scrollCooldown {
		return m, nil
	}

	m.lastScrollTime = now

	// Queue画面がフォーカスされている場合
	if m.queueFocused && m.showQueue {
		maxQueueIndex := len(m.playerState.List) - 1
		if m.queueSelectedIndex < maxQueueIndex {
			m.queueSelectedIndex++
			// Adjust scroll to keep selection visible
			// Queue表示の高さを計算（コンテンツエリアの1/3）
			contentAreaHeight := m.height - m.playerHeight

			queueHeight := max(contentAreaHeight/3, 5)

			visibleLines := max(
				// Header, spacing, and scroll indicator
				queueHeight-4, 1)

			if m.queueSelectedIndex >= m.queueScrollOffset+visibleLines {
				m.queueScrollOffset = m.queueSelectedIndex - visibleLines + 1
			}
		}

		return m, nil
	}

	// 通常のコンテンツスクロール
	switch m.state {
	case PlaylistDetailView:
		if m.playlistSelectedIndex < len(m.playlistTracks)-1 {
			m.playlistSelectedIndex++
			m.adjustPlaylistScroll()
		}
	default:
		maxIndex := m.getMaxIndex()
		if m.selectedIndex < maxIndex {
			m.selectedIndex++
			m.adjustScroll()
		}
	}

	return m, nil
}

// ビューごとの最大アイテム数を取得.
func (m *Model) getMaxItems() int {
	switch m.state {
	case PlaylistDetailView:
		return len(m.playlistTracks)
	case SearchView:
		return len(m.searchResults)
	case HomeView:
		if m.currentSectionIndex < len(m.sections) {
			return len(m.sections[m.currentSectionIndex].Contents)
		}

		return 0
	case PlaylistListView:
		return len(m.playlists)
	default:
		return 0
	}
}

// 選択されたトラックを再生.
func (m *Model) playSelectedTrack() (tea.Model, tea.Cmd) {
	switch m.state {
	case PlaylistDetailView:
		if len(m.playlistTracks) > 0 && m.playlistSelectedIndex < len(m.playlistTracks) {
			// Clear the current queue
			m.systems.Player.SendAction(structures.CleanupAction{})

			// Add all tracks from the selected position onwards
			tracksToAdd := m.playlistTracks[m.playlistSelectedIndex:]
			m.systems.Player.SendAction(structures.AddTracksToQueueAction{Tracks: tracksToAdd})

			// Start playing
			m.systems.Player.SendAction(structures.PlayAction{})
		}
	case SearchView:
		if len(m.searchResults) > 0 && m.selectedIndex < len(m.searchResults) {
			track := m.searchResults[m.selectedIndex]
			m.systems.Player.SendAction(structures.CleanupAction{})
			m.systems.Player.SendAction(structures.AddTrackAction{Track: track})
			m.systems.Player.SendAction(structures.PlayAction{})
		}
	}

	return m, nil
}
