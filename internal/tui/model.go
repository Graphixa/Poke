package tui

import (
	"context"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/search"
)

// Options configures a browse session.
type Options struct {
	Index    *search.Index
	Details  DetailSource
	Sprites  SpriteRenderer
	Selected *domain.SpeciesSummary
	Prefill  string
	Context  context.Context
}

// Model is the Bubble Tea browse state machine.
type Model struct {
	ctx     context.Context
	index   *search.Index
	details DetailSource
	sprites SpriteRenderer

	width  int
	height int

	query    string
	filtered []domain.SpeciesSummary
	cursor   int
	listOff  int

	activeTab   Tab
	detailOff   int
	detailLines []string

	selectionGen int
	trackedID    int
	resources    map[int]*resourceCache

	spriteCancel context.CancelFunc
	spriteReqID  int

	showHelp bool
	quitting bool
}

// NewModel builds browse state from options.
func NewModel(opts Options) Model {
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	m := Model{
		ctx:       ctx,
		index:     opts.Index,
		details:   opts.Details,
		sprites:   opts.Sprites,
		resources: make(map[int]*resourceCache),
		activeTab: TabStats,
	}
	if opts.Prefill != "" {
		m.query = opts.Prefill
	}
	m.refilter()
	if opts.Selected != nil {
		m.selectSpeciesID(opts.Selected.ID)
	} else if len(m.filtered) > 0 {
		m.cursor = 0
	}
	if sel, ok := m.selected(); ok {
		m.trackedID = sel.ID
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return m.ensureSelectionLoads()
}

func (m *Model) layout() LayoutFlags {
	return layoutFor(m.width, m.height)
}

func (m *Model) selected() (domain.SpeciesSummary, bool) {
	if len(m.filtered) == 0 || m.cursor < 0 || m.cursor >= len(m.filtered) {
		return domain.SpeciesSummary{}, false
	}
	return m.filtered[m.cursor], true
}

func (m *Model) currentResource() *resourceCache {
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	return m.resources[sel.ID]
}

func (m *Model) refilter() {
	var prevID int
	if sel, ok := m.selected(); ok {
		prevID = sel.ID
	}
	if m.index == nil {
		m.filtered = nil
		m.cursor = 0
		m.listOff = 0
		return
	}
	m.filtered = m.index.Search(m.query)
	if len(m.filtered) == 0 {
		m.cursor = 0
		m.listOff = 0
		return
	}
	if prevID != 0 {
		for i, s := range m.filtered {
			if s.ID == prevID {
				m.cursor = i
				m.ensureListVisible()
				return
			}
		}
	}
	m.cursor = 0
	m.listOff = 0
}

func (m *Model) selectSpeciesID(id int) {
	for i, s := range m.filtered {
		if s.ID == id {
			m.cursor = i
			m.ensureListVisible()
			return
		}
	}
	// Selected species may be outside the current filter; clear filter to show it.
	if m.index != nil {
		m.query = ""
		m.filtered = m.index.Search("")
		for i, s := range m.filtered {
			if s.ID == id {
				m.cursor = i
				m.ensureListVisible()
				return
			}
		}
	}
}

func (m *Model) ensureListVisible() {
	visible := m.listVisibleRows()
	if visible <= 0 {
		m.listOff = 0
		return
	}
	if m.cursor < m.listOff {
		m.listOff = m.cursor
	}
	if m.cursor >= m.listOff+visible {
		m.listOff = m.cursor - visible + 1
	}
	if m.listOff < 0 {
		m.listOff = 0
	}
}

func (m *Model) listVisibleRows() int {
	lay := m.layout()
	h := m.height
	if h <= 0 {
		h = 30
	}
	// Title + footer + search + optional sprite + borders.
	rows := h - 6
	if lay.ShowSprite {
		rows -= m.layout().spriteSide() + 1
	}
	if lay.OnePane {
		rows = h - 5
	}
	if rows < 3 {
		rows = 3
	}
	return rows
}

func (m *Model) moveCursor(delta int) {
	if len(m.filtered) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	m.detailOff = 0
	m.ensureListVisible()
}

func (m *Model) setQuery(q string) {
	m.query = q
	m.refilter()
	m.detailOff = 0
}

func (m *Model) appendRune(r rune) {
	var buf [4]byte
	n := utf8.EncodeRune(buf[:], r)
	m.setQuery(m.query + string(buf[:n]))
}

func (m *Model) backspace() {
	if m.query == "" {
		return
	}
	_, size := utf8.DecodeLastRuneInString(m.query)
	m.setQuery(m.query[:len(m.query)-size])
}

func (m *Model) clearQuery() {
	if m.query == "" {
		return
	}
	m.setQuery("")
}

func (m *Model) canQuitWithQ() bool {
	return m.query == ""
}

func (m *Model) bumpSelection() {
	m.selectionGen++
	m.detailOff = 0
	m.cancelSprite()
}

func (m *Model) ensureResource(id int) *resourceCache {
	if m.resources == nil {
		m.resources = make(map[int]*resourceCache)
	}
	r, ok := m.resources[id]
	if !ok {
		r = &resourceCache{}
		m.resources[id] = r
	}
	return r
}
