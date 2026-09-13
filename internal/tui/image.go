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
	m.clearImagePane()
}

func (m *Model) setImageLoading() {
	m.clearImagePane()
	m.imageState = ImageLoading
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
	if res.Native {
		m.markNativeOverlayDirty()
	}
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
	for len(m.resources) >= detailCacheLimit && len(m.lruIDs) > 0 {
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
