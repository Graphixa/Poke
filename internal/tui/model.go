package tui

import (
	"context"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/search"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

// Options configures a browse session.
type Options struct {
	Index    *search.Index
	Details  DetailSource
	Sprites  SpriteRenderer
	Selected *domain.SpeciesSummary
	Prefill  string
	Context  context.Context
	// DisableSprite forces the sprite panel off regardless of terminal size.
	DisableSprite bool
	// Warmer is an optional priority sprite prewarm service.
	Warmer SpriteWarmer
	// DossierWarmer is an optional priority dossier prewarm service.
	DossierWarmer DossierWarmer
	// WarmEvents receives progress updates from the warmer (optional).
	WarmEvents <-chan WarmProgress
}

// SpriteWarmer prioritizes background chafa precomputation.
type SpriteWarmer interface {
	Prioritize(speciesID int)
	SetNeighborsFromList(list []domain.SpeciesSummary, cursor, radius int)
	SetCells(cells int)
	Snapshot() (done, total int)
}

// DossierWarmer prioritizes background dossier assembly.
type DossierWarmer interface {
	Prioritize(speciesID int)
	SetNeighborsFromList(list []domain.SpeciesSummary, cursor, radius int)
}

// WarmProgress is a TUI-friendly warm status snapshot.
type WarmProgress struct {
	Done    int
	Total   int
	Message string
}

// Model is the Bubble Tea browse state machine.
type Model struct {
	ctx     context.Context
	index   *search.Index
	details DetailSource
	sprites SpriteRenderer

	layoutTh layoutThresholds

	width  int
	height int

	keys   keyMap
	help   help.Model
	search textinput.Model

	filtered []domain.SpeciesSummary
	cursor   int
	listOff  int

	browseView    BrowseView
	dialogSection DialogSection
	dialogVP      viewport.Model

	selectionGen int
	trackedID    int
	resources    map[int]*resourceCache

	spriteCancel      context.CancelFunc
	spriteReqID       int
	spriteLoadSpecies int
	nativePaintPending bool

	warmer        SpriteWarmer
	dossierWarmer DossierWarmer
	warmEvents    <-chan WarmProgress
	warmDone      int
	warmTotal     int
	warmActive    bool

	showHelp   bool
	showDialog bool

	quitting bool
}

// NewModel builds browse state from options.
func NewModel(opts Options) Model {
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	m := Model{
		ctx:     ctx,
		index:   opts.Index,
		details: opts.Details,
		sprites: opts.Sprites,
		layoutTh: layoutThresholds{
			DisableSprite: opts.DisableSprite,
		},
		resources:     make(map[int]*resourceCache),
		browseView:    BrowseSprite,
		dialogSection: DialogSummary,
		keys:          defaultKeyMap(),
		help:          newHelpModel(),
		search:        newSearchInput(),
		dialogVP:      newContentViewport(),
		warmer:        opts.Warmer,
		dossierWarmer: opts.DossierWarmer,
		warmEvents:    opts.WarmEvents,
	}
	if opts.Prefill != "" {
		m.search.SetValue(opts.Prefill)
		m.search.CursorEnd()
	}
	_ = m.search.Focus()
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

func newSearchInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "Search Pokémon…"
	ti.CharLimit = 64
	ti.SetVirtualCursor(false)
	p := style.CurrentPalette()
	s := textinput.DefaultDarkStyles()
	placeholder := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Placeholders)).Faint(true)
	prompt := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Components))
	s.Focused.Placeholder = placeholder
	s.Blurred.Placeholder = placeholder
	s.Focused.Prompt = prompt
	s.Blurred.Prompt = prompt
	s.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Components))
	s.Blurred.Text = s.Focused.Text
	ti.SetStyles(s)
	ti.KeyMap.LineStart.SetEnabled(false)
	ti.KeyMap.LineEnd.SetEnabled(false)
	return ti
}

func newHelpModel() help.Model {
	h := help.New()
	h.ShortSeparator = " • "
	h.Styles = help.DefaultDarkStyles()
	p := style.CurrentPalette()
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Placeholders)).Faint(true)
	h.Styles.ShortKey = muted
	h.Styles.ShortDesc = muted
	h.Styles.ShortSeparator = muted
	h.Styles.FullKey = muted
	h.Styles.FullDesc = muted
	h.Styles.FullSeparator = muted
	h.Styles.Ellipsis = muted
	return h
}

func newContentViewport() viewport.Model {
	vp := viewport.New()
	vp.MouseWheelEnabled = false
	vp.KeyMap.PageDown.SetEnabled(false)
	vp.KeyMap.PageUp.SetEnabled(false)
	vp.KeyMap.HalfPageUp.SetEnabled(false)
	vp.KeyMap.HalfPageDown.SetEnabled(false)
	vp.KeyMap.Up.SetEnabled(false)
	vp.KeyMap.Down.SetEnabled(false)
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)
	return vp
}

func (m Model) Init() tea.Cmd {
	m.notifyWarmer()
	return tea.Batch(m.ensureSelectionLoads(), listenWarmProgress(m.warmEvents))
}

func (m *Model) layout() LayoutFlags {
	return layoutFor(m.width, m.height, m.layoutTh)
}

func (m *Model) queryValue() string {
	return m.search.Value()
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
	m.filtered = m.index.Search(m.queryValue())
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
	if m.index != nil {
		m.search.SetValue("")
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
	rows := h - 6
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
	m.browseView = BrowseSprite
	m.dialogVP.GotoTop()
	m.ensureListVisible()
}

func (m *Model) clearQuery() {
	if m.queryValue() == "" {
		return
	}
	m.search.SetValue("")
	m.refilter()
	m.dialogVP.GotoTop()
}

func (m *Model) canQuitWithQ() bool {
	return m.queryValue() == ""
}

func (m *Model) bumpSelection() {
	m.selectionGen++
	m.browseView = BrowseSprite
	m.showDialog = false
	m.dialogVP.GotoTop()
	m.cancelSprite()
	if m.sprites != nil {
		m.sprites.ClearNative()
	}
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

func (m *Model) syncDialogViewport() {
	lines := m.dialogContentLines()
	m.dialogVP.SetContentLines(lines)
	if m.dialogVP.Height() < 1 {
		m.dialogVP.SetHeight(m.dialogPageSize())
	}
}

func (m *Model) dialogPageSize() int {
	h := m.height - 8
	if h < 6 {
		h = 6
	}
	return h
}
