package tui

import "github.com/Graphixa/Pokedex/internal/domain"

// Exported accessors for model/layout tests.

func (m Model) Query() string      { return m.queryValue() }
func (m Model) CanQuitWithQ() bool { return m.canQuitWithQ() }
func (m Model) HelpVisible() bool  { return m.showHelp }
func (m Model) DialogVisible() bool { return m.showDialog }
func (m Model) Quitting() bool     { return m.quitting }
func (m Model) ActiveBrowseView() BrowseView { return m.browseView }
func (m Model) ActiveDialogSection() DialogSection { return m.dialogSection }
func (m Model) DialogOffset() int  { return m.dialogVP.YOffset() }
func (m Model) SelectionGen() int  { return m.selectionGen }
func (m Model) Width() int         { return m.width }
func (m Model) Height() int        { return m.height }

func (m Model) SelectedID() (int, bool) {
	sel, ok := m.selected()
	if !ok {
		return 0, false
	}
	return sel.ID, true
}

func (m Model) ResourceName(speciesID int) string {
	r := m.resources[speciesID]
	if r == nil || !r.HasCore {
		return ""
	}
	return r.Pokemon.DisplayName
}

func (m Model) LayoutFlags() LayoutFlags { return m.layout() }

// CoreMsgForTest builds a core-loaded message for race tests.
func CoreMsgForTest(gen, speciesID int, pokemon domain.Pokemon, species domain.Species, err error) coreMsg {
	return coreMsg{gen: gen, speciesID: speciesID, pokemon: pokemon, species: species, err: err}
}

// SpriteMsgForTest builds a sprite-loaded message for race tests.
func SpriteMsgForTest(reqID, gen, speciesID, cells int, ansi string, placeholder bool) spriteMsg {
	return spriteMsg{
		reqID: reqID, gen: gen, speciesID: speciesID, cells: cells,
		ansi: ansi, placeholder: placeholder,
	}
}

// SpriteReqID exposes the current sprite request id for tests.
func (m Model) SpriteReqID() int { return m.spriteReqID }

// HasSpriteANSI reports whether the selected species has rendered sprite art.
func (m Model) HasSpriteANSI() bool {
	sel, ok := m.selected()
	if !ok {
		return false
	}
	r := m.resources[sel.ID]
	return r != nil && r.HasSprite && r.SpriteANSI != ""
}

// CurrentSpriteCells exposes the active sprite cell size for tests.
func (m Model) CurrentSpriteCells() int { return m.currentSpriteCells() }
