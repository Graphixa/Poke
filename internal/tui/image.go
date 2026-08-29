package tui

import (
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// ImageState is the sprite pane visual state machine.
type ImageState int

const (
	ImageBlank ImageState = iota
	ImageLoading
	ImageReady
	ImageUnavailable
)

const detailCacheLimit = 16

func (m *Model) clearImagePane() {
	m.imageANSI = ""
	m.imageNative = false
	m.imageKind = ""
	m.imageCells = 0
	m.imageSpeciesID = 0
	m.imagePlaceholder = false
	m.imageLoading = false
}

func (m *Model) setImageBlank() {
	m.imageState = ImageBlank
	m.imageSpinner = false
	m.clearImagePane()
}

func (m *Model) setImageLoading() {
	m.clearImagePane()
	m.imageState = ImageLoading
	m.imageSpinner = true
	m.imageLoading = true
}

func (m *Model) applySpriteFrame(speciesID, cells int, res sprite.RenderResult) {
	m.imageLoading = false
	m.imageSpeciesID = speciesID
	m.imageCells = cells
	if res.UsePlaceholder || res.ANSI == "" {
		m.imageANSI = ""
		m.imageNative = false
		m.imageKind = ""
		m.imagePlaceholder = true
		m.imageState = ImageUnavailable
		m.imageSpinner = false
		if m.isSelectedID(speciesID) {
			m.markNativeOverlayDirty()
		}
		return
	}
	m.imageANSI = res.ANSI
	m.imageNative = res.Native
	m.imageKind = res.AssetKind
	m.imagePlaceholder = false
	m.imageState = ImageReady
	m.imageSpinner = false
	if res.Native {
		m.markNativeOverlayDirtyFollowUp()
	}
}

func (m Model) spinnerView() string {
	return m.spinner.View()
}

func (m *Model) touchResource(id int) {
	if id < 1 {
		return
	}
	out := m.lruIDs[:0]
	for _, existing := range m.lruIDs {
		if existing != id {
			out = append(out, existing)
		}
	}
	m.lruIDs = append(out, id)
}

func (m *Model) evictResources(keepID int) {
	if len(m.resources) <= detailCacheLimit {
		return
	}
	for len(m.resources) > detailCacheLimit && len(m.lruIDs) > 0 {
		oldest := m.lruIDs[0]
		m.lruIDs = m.lruIDs[1:]
		if oldest == keepID {
			m.lruIDs = append(m.lruIDs, oldest)
			if len(m.lruIDs) == 1 {
				return
			}
			continue
		}
		delete(m.resources, oldest)
	}
}
