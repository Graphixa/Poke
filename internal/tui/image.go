package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// ImageState is the sprite pane visual state machine.
type ImageState int

const (
	ImageBlank ImageState = iota
	ImageLoading
	ImageReady
	ImageUnavailable
)

const (
	selectionDebounce = 50 * time.Millisecond
	spinnerThreshold  = 100 * time.Millisecond
	detailCacheLimit  = 16
)

type selectionDebounceMsg struct {
	Generation int
	PokemonID  int
}

type spinnerDelayMsg struct {
	Generation int
	PokemonID  int
}

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

func (m *Model) scheduleSelectionDebounce() tea.Cmd {
	m.setImageBlank()
	m.cancelSprite()
	if m.sprites != nil {
		m.sprites.ClearNative()
	}
	gen := m.selectionGen
	id := 0
	if sel, ok := m.selected(); ok {
		id = sel.ID
	}
	return tea.Tick(selectionDebounce, func(time.Time) tea.Msg {
		return selectionDebounceMsg{Generation: gen, PokemonID: id}
	})
}

func (m *Model) scheduleSpinnerDelay(gen, pokemonID int) tea.Cmd {
	return tea.Tick(spinnerThreshold, func(time.Time) tea.Msg {
		return spinnerDelayMsg{Generation: gen, PokemonID: pokemonID}
	})
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
