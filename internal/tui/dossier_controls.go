package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Graphixa/Poke/internal/domain"
)

func (m *Model) dialogHasForms() bool {
	return len(m.dialogForms) > 1
}

func (m *Model) dialogOverlayOpen() bool {
	return m.gamePickerOpen || m.formPickerOpen || m.methodPickerOpen
}

// dialogContextLines builds notice rows for the chrome.
// Form/Game/Method live as top-border chips (or are hidden); picker code stays.
func (m Model) dialogContextLines(width int) []string {
	_ = width
	if m.dialogNotice == "" {
		return nil
	}
	return []string{m.dialogNotice}
}

func (m *Model) exitMoveDetail() {
	m.moveDetailOpen = false
}

func (m *Model) saveDialogScroll() {
	if m.dialogScroll == nil {
		m.dialogScroll = map[DialogSection]int{}
	}
	m.dialogScroll[m.dialogSection] = m.dialogVP.YOffset()
}

func (m *Model) restoreDialogScroll() {
	off := 0
	if m.dialogScroll != nil {
		off = m.dialogScroll[m.dialogSection]
	}
	m.syncDialogViewport()
	if off > 0 {
		m.dialogVP.SetYOffset(off)
	} else {
		m.dialogVP.GotoTop()
	}
}

func (m *Model) changeDialogSection(next DialogSection) {
	m.saveDialogScroll()
	m.moveDetailOpen = false
	m.dialogSection = next
	m.ensureMoveMethod()
	m.restoreDialogScroll()
	// No automatic game picker — unset game shows in-content prompt.
}

func (m *Model) initDialogContext(speciesID int, core domain.Dossier) {
	m.moveDetailOpen = false
	m.dialogNotice = ""
	m.dialogFormIdx = 0
	m.dialogForms = nil
	m.dialogFormID = 0
	m.dialogPokemon = core.Pokemon
	m.dialogMoveMethod = ""
	m.dialogSpeciesID = speciesID
	if m.catalog != nil {
		if forms, err := m.catalog.SelectableFormsForSpecies(speciesID); err == nil && len(forms) > 0 {
			m.dialogForms = forms
			for i, f := range forms {
				if f.IsDefault {
					m.dialogFormIdx = i
					m.dialogFormID = f.FormID
					break
				}
			}
		}
	}
	m.applyResolvedContext(false)
	m.refreshDialogGameLabel()
	if m.dialogGameID > 0 && !m.dialogGameValid() {
		m.dialogGameID = 0
		m.dialogGameLabel = ""
		m.dialogGameSlug = ""
	}
	m.ensureMoveMethod()
	m.encounterFocusIdx = 0
}

func (m *Model) dialogGameValid() bool {
	if m.dialogGameID < 1 || m.catalog == nil {
		return false
	}
	vers, err := m.catalog.CatalogVersions()
	if err != nil {
		return false
	}
	for _, v := range vers {
		if v.ID == m.dialogGameID {
			return true
		}
	}
	return false
}

func (m *Model) refreshDialogGameLabel() {
	m.dialogGameLabel = ""
	m.dialogGameSlug = ""
	if m.dialogGameID < 1 || m.catalog == nil {
		return
	}
	vers, err := m.catalog.CatalogVersions()
	if err != nil {
		return
	}
	for _, v := range vers {
		if v.ID == m.dialogGameID {
			m.dialogGameSlug = v.Slug
			m.dialogGameLabel = v.DisplayName
			if m.dialogGameLabel == "" {
				m.dialogGameLabel = v.Slug
			}
			return
		}
	}
}

func (m *Model) currentChoice() domain.SelectableForm {
	if len(m.dialogForms) == 0 {
		return domain.SelectableForm{PokemonID: m.dialogPokemon.ID, IsDefault: true, DisplayName: "Default"}
	}
	if m.dialogFormIdx < 0 || m.dialogFormIdx >= len(m.dialogForms) {
		m.dialogFormIdx = 0
	}
	return m.dialogForms[m.dialogFormIdx]
}

func (m *Model) applyResolvedContext(resetAffectedScroll bool) {
	if m.catalog == nil || m.dialogSpeciesID < 1 {
		m.applyDialogFormPokemon()
		return
	}
	ch := m.currentChoice()
	resolved, err := m.catalog.ResolveDossierContext(m.dialogSpeciesID, ch.PokemonID, ch.FormID, m.dialogGameID)
	if err != nil {
		m.applyDialogFormPokemon()
		return
	}
	if resolved.FellBackToDefault {
		m.dialogNotice = resolved.Notice
		for i, f := range m.dialogForms {
			if f.IsDefault {
				m.dialogFormIdx = i
				m.dialogFormID = f.FormID
				break
			}
		}
	} else if m.dialogNotice != "" && !resolved.FellBackToDefault {
		// keep notice until next nav clears it elsewhere
	}
	m.dialogFormID = resolved.FormID
	if resolved.PokemonID > 0 && (m.dialogPokemon.ID != resolved.PokemonID || m.dialogPokemon.ID == 0) {
		if p, err := m.catalog.PokemonByID(m.ctx, resolved.PokemonID); err == nil {
			m.dialogPokemon = p
		}
	}
	if resetAffectedScroll && m.dialogScroll != nil {
		for _, sec := range []DialogSection{DialogMoves, DialogEncounters, DialogDescriptions, DialogOverview, DialogStats, DialogBiology} {
			m.dialogScroll[sec] = 0
		}
	}
	m.ensureMoveMethod()
}

func (m *Model) applyDialogFormPokemon() {
	if len(m.dialogForms) == 0 {
		return
	}
	f := m.dialogForms[m.dialogFormIdx]
	m.dialogFormID = f.FormID
	if f.PokemonID < 1 || m.catalog == nil {
		return
	}
	if m.dialogPokemon.ID == f.PokemonID && m.dialogPokemon.ID > 0 {
		return
	}
	p, err := m.catalog.PokemonByID(m.ctx, f.PokemonID)
	if err != nil {
		return
	}
	m.dialogPokemon = p
}

func (m *Model) selectDialogForm(idx int) {
	if idx < 0 || idx >= len(m.dialogForms) {
		return
	}
	optCompat := domain.FormCompatSupported
	if m.catalog != nil {
		if opts, err := m.catalog.FormOptionsForGame(m.dialogSpeciesID, m.dialogGameID); err == nil {
			for _, o := range opts {
				if o.Form.Key == m.dialogForms[idx].Key {
					optCompat = o.Compat
					break
				}
			}
		}
	}
	if optCompat == domain.FormCompatUnsupported {
		return // picker should not confirm these
	}
	m.dialogFormIdx = idx
	m.dialogFormID = m.dialogForms[idx].FormID
	m.dialogNotice = ""
	m.moveDetailOpen = false
	m.applyResolvedContext(true)
	m.syncDialogViewport()
	m.dialogVP.GotoTop()
}

func (m *Model) openGamePicker() {
	m.moveDetailOpen = false
	m.formPickerOpen = false
	m.methodPickerOpen = false
	m.dialogCatalogErr = ""
	if m.catalog == nil {
		m.dialogCatalogErr = "Game catalog unavailable. Rebuild the dataset cache."
		m.gamePickerOpen = true
		m.gamePickerItems = nil
		return
	}
	vers, err := m.catalog.CatalogVersions()
	if err != nil {
		m.dialogCatalogErr = "Game catalog failed to load. Rebuild the dataset cache."
		m.gamePickerOpen = true
		m.gamePickerItems = nil
		return
	}
	if len(vers) == 0 {
		m.dialogCatalogErr = "No games in dataset yet. Wait for cache rebuild to finish, then retry G."
		m.gamePickerOpen = true
		m.gamePickerItems = nil
		return
	}
	m.gamePickerOpen = true
	m.gamePickerFilter = ""
	m.gamePickerItems = withAllGameOption(vers)
	m.gamePickerSuggest = 0
	m.gamePickerCursor = 0
	if m.dialogGameID > 0 {
		for i, v := range m.gamePickerItems {
			if v.ID == m.dialogGameID {
				m.gamePickerCursor = i
				break
			}
		}
	}
}

func (m *Model) closeGamePicker(confirm bool) {
	if !m.gamePickerOpen {
		return
	}
	if confirm && len(m.gamePickerItems) > 0 && m.dialogCatalogErr == "" {
		if m.gamePickerCursor < 0 {
			m.gamePickerCursor = 0
		}
		if m.gamePickerCursor >= len(m.gamePickerItems) {
			m.gamePickerCursor = len(m.gamePickerItems) - 1
		}
		v := m.gamePickerItems[m.gamePickerCursor]
		if v.ID == 0 {
			m.dialogGameID = 0
			m.dialogGameSlug = ""
			m.dialogGameLabel = ""
		} else {
			m.dialogGameID = v.ID
			m.dialogGameSlug = v.Slug
			m.dialogGameLabel = v.DisplayName
			if m.dialogGameLabel == "" {
				m.dialogGameLabel = v.Slug
			}
		}
		if m.saveGameVersion != nil {
			_ = m.saveGameVersion(v.ID)
		}
		m.applyResolvedContext(true)
		m.encounterFocusIdx = 0
		m.moveFocusIdx = 0
	}
	m.gamePickerOpen = false
	m.gamePickerFilter = ""
	m.syncDialogViewport()
}

func (m *Model) filterGamePicker() {
	if m.catalog == nil {
		return
	}
	all, err := m.catalog.CatalogVersions()
	if err != nil {
		return
	}
	q := strings.ToLower(strings.TrimSpace(m.gamePickerFilter))
	if q == "" {
		m.gamePickerItems = withAllGameOption(all)
	} else {
		var out []domain.Version
		if strings.Contains("all", q) || strings.Contains("(all)", q) {
			out = append(out, allGameOption())
		}
		for _, v := range all {
			name := strings.ToLower(v.DisplayName + " " + v.Slug)
			if strings.Contains(name, q) {
				out = append(out, v)
			}
		}
		m.gamePickerItems = out
	}
	if m.gamePickerCursor >= len(m.gamePickerItems) {
		m.gamePickerCursor = max(0, len(m.gamePickerItems)-1)
	}
}

func allGameOption() domain.Version {
	return domain.Version{ID: 0, Slug: "all", DisplayName: "(All)"}
}

func withAllGameOption(vers []domain.Version) []domain.Version {
	return append([]domain.Version{allGameOption()}, vers...)
}

func (m *Model) openFormPicker() {
	if !m.dialogHasForms() {
		return
	}
	m.moveDetailOpen = false
	m.gamePickerOpen = false
	m.methodPickerOpen = false
	m.dialogCatalogErr = ""
	m.formPickerOpen = true
	m.formPickerFilter = ""
	m.formPickerCursor = m.dialogFormIdx
	m.refreshFormPickerItems()
}

func (m *Model) refreshFormPickerItems() {
	m.formPickerItems = nil
	if m.catalog == nil {
		for _, f := range m.dialogForms {
			m.formPickerItems = append(m.formPickerItems, domain.FormOption{Form: f, Compat: domain.FormCompatUnknown})
		}
		return
	}
	opts, err := m.catalog.FormOptionsForGame(m.dialogSpeciesID, m.dialogGameID)
	if err != nil || len(opts) == 0 {
		for _, f := range m.dialogForms {
			m.formPickerItems = append(m.formPickerItems, domain.FormOption{Form: f, Compat: domain.FormCompatSupported})
		}
		return
	}
	q := strings.ToLower(strings.TrimSpace(m.formPickerFilter))
	for _, o := range opts {
		if q != "" {
			name := strings.ToLower(o.Form.DisplayName + " " + o.Form.Slug)
			if !strings.Contains(name, q) {
				continue
			}
		}
		m.formPickerItems = append(m.formPickerItems, o)
	}
	if m.formPickerCursor >= len(m.formPickerItems) {
		m.formPickerCursor = max(0, len(m.formPickerItems)-1)
	}
}

func (m *Model) closeFormPicker(confirm bool) {
	if !m.formPickerOpen {
		return
	}
	if confirm && len(m.formPickerItems) > 0 {
		o := m.formPickerItems[m.formPickerCursor]
		if o.Compat != domain.FormCompatUnsupported {
			for i, f := range m.dialogForms {
				if f.Key == o.Form.Key {
					m.selectDialogForm(i)
					break
				}
			}
		}
	}
	m.formPickerOpen = false
	m.formPickerFilter = ""
	m.syncDialogViewport()
}

func (m *Model) openMethodPicker() {
	if m.dialogSection != DialogMoves {
		return
	}
	m.moveDetailOpen = false
	m.gamePickerOpen = false
	m.formPickerOpen = false
	m.methodPickerOpen = true
	m.methodPickerItems = m.availableMoveMethods()
	m.methodPickerCursor = 0
	for i, meth := range m.methodPickerItems {
		if meth == m.dialogMoveMethod {
			m.methodPickerCursor = i
			break
		}
	}
}

func (m *Model) closeMethodPicker(confirm bool) {
	if !m.methodPickerOpen {
		return
	}
	if confirm && len(m.methodPickerItems) > 0 {
		m.dialogMoveMethod = m.methodPickerItems[m.methodPickerCursor]
		m.moveFocusIdx = 0
		if m.dialogScroll != nil {
			m.dialogScroll[DialogMoves] = 0
		}
	}
	m.methodPickerOpen = false
	m.syncDialogViewport()
}

func (m *Model) availableMoveMethods() []string {
	rows := m.dialogMoveRowsAll()
	seen := map[string]struct{}{}
	var methods []string
	order := []string{"level-up", "machine", "egg", "tutor"}
	for _, row := range rows {
		meth := row.Method
		if meth == "" {
			meth = "level-up"
		}
		if _, ok := seen[meth]; ok {
			continue
		}
		seen[meth] = struct{}{}
	}
	for _, meth := range order {
		if _, ok := seen[meth]; ok {
			methods = append(methods, meth)
			delete(seen, meth)
		}
	}
	for meth := range seen {
		methods = append(methods, meth)
	}
	if len(methods) == 0 {
		return []string{"level-up"}
	}
	return methods
}

func (m *Model) ensureMoveMethod() {
	methods := m.availableMoveMethods()
	if len(methods) == 0 {
		m.dialogMoveMethod = "level-up"
		return
	}
	for _, meth := range methods {
		if meth == m.dialogMoveMethod {
			return
		}
	}
	m.dialogMoveMethod = methods[0]
}

func (m *Model) currentFormPokemonID(fallback int) int {
	if len(m.dialogForms) > 0 {
		id := m.dialogForms[m.dialogFormIdx].PokemonID
		if id > 0 {
			return id
		}
	}
	if m.dialogPokemon.ID > 0 {
		return m.dialogPokemon.ID
	}
	return fallback
}

func (m *Model) nudgeMoveFocus(delta int) {
	rows := m.dialogMoveRows()
	if len(rows) == 0 {
		return
	}
	m.moveFocusIdx += delta
	if m.moveFocusIdx < 0 {
		m.moveFocusIdx = 0
	}
	if m.moveFocusIdx >= len(rows) {
		m.moveFocusIdx = len(rows) - 1
	}
	m.lastMoveFocusIdx = m.moveFocusIdx
}

// ensureMoveFocusVisible scrolls the dossier viewport so the focused move row stays on screen.
func (m *Model) ensureMoveFocusVisible() {
	vis := m.dialogPageSize()
	if vis <= 0 || m.moveFocusIdx < 0 {
		return
	}
	off := m.dialogVP.YOffset()
	if m.moveFocusIdx < off {
		m.dialogVP.SetYOffset(m.moveFocusIdx)
		return
	}
	if m.moveFocusIdx >= off+vis {
		m.dialogVP.SetYOffset(m.moveFocusIdx - vis + 1)
	}
}

func (m Model) dialogEncounterRows() []domain.Encounter {
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	r := m.resources[sel.ID]
	if r == nil {
		return nil
	}
	rows := r.Encounters
	if m.dialogGameID >= 1 {
		var filtered []domain.Encounter
		for _, e := range rows {
			if e.VersionID == m.dialogGameID || (m.dialogGameSlug != "" && e.Version == m.dialogGameSlug) {
				filtered = append(filtered, e)
			}
		}
		rows = filtered
	}
	return sortEncountersByGameOrder(rows, m.versionOrderIndex())
}

// versionOrderIndex maps version id/slug → catalog Order (game picker order).
func (m Model) versionOrderIndex() map[string]int {
	out := map[string]int{}
	if m.catalog == nil {
		return out
	}
	vers, err := m.catalog.CatalogVersions()
	if err != nil {
		return out
	}
	for i, v := range vers {
		ord := v.Order
		if ord == 0 {
			ord = i + 1
		}
		if v.ID > 0 {
			out[fmt.Sprintf("id:%d", v.ID)] = ord
		}
		if v.Slug != "" {
			out["slug:"+v.Slug] = ord
		}
	}
	return out
}

func sortEncountersByGameOrder(rows []domain.Encounter, order map[string]int) []domain.Encounter {
	if len(rows) < 2 {
		return rows
	}
	out := append([]domain.Encounter(nil), rows...)
	key := func(e domain.Encounter) int {
		if e.VersionID > 0 {
			if o, ok := order[fmt.Sprintf("id:%d", e.VersionID)]; ok {
				return o
			}
		}
		if e.Version != "" {
			if o, ok := order["slug:"+e.Version]; ok {
				return o
			}
		}
		if e.VersionGroupOrder > 0 {
			return e.VersionGroupOrder
		}
		return 1 << 30
	}
	sort.SliceStable(out, func(i, j int) bool {
		oi, oj := key(out[i]), key(out[j])
		if oi != oj {
			return oi < oj
		}
		if out[i].Area != out[j].Area {
			return out[i].Area < out[j].Area
		}
		return out[i].Method < out[j].Method
	})
	return out
}

func (m *Model) nudgeEncounterFocus(delta int) {
	rows := m.dialogEncounterRows()
	if len(rows) == 0 {
		return
	}
	m.encounterFocusIdx += delta
	if m.encounterFocusIdx < 0 {
		m.encounterFocusIdx = 0
	}
	if m.encounterFocusIdx >= len(rows) {
		m.encounterFocusIdx = len(rows) - 1
	}
}

func (m *Model) ensureEncounterFocusVisible() {
	rows := m.dialogEncounterRows()
	if len(rows) == 0 || m.encounterFocusIdx < 0 {
		return
	}
	_, mainLine := formatEncounterTableRowsMapped(rows, m.dialogGeom().BodyW)
	if m.encounterFocusIdx >= len(mainLine) {
		return
	}
	line := mainLine[m.encounterFocusIdx]
	vis := m.dialogPageSize()
	if vis <= 0 {
		return
	}
	off := m.dialogVP.YOffset()
	if line < off {
		m.dialogVP.SetYOffset(line)
		return
	}
	if line >= off+vis {
		m.dialogVP.SetYOffset(line - vis + 1)
	}
}

func (m *Model) dialogMoveRowsAll() []domain.LevelUpMove {
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	r := m.resources[sel.ID]
	if r == nil {
		return nil
	}
	if m.catalog == nil {
		return r.Moves
	}
	pokeID := m.currentFormPokemonID(r.Pokemon.ID)
	if m.dialogGameID > 0 {
		if vg, ok := m.catalog.VersionGroupIDForVersion(m.dialogGameID); ok {
			if rows, err := m.catalog.MovesForGroup(pokeID, vg); err == nil && len(rows) > 0 {
				return rows
			}
			if rows, err := m.catalog.LevelUpMovesForGroup(pokeID, vg); err == nil {
				return rows
			}
		}
		return r.Moves
	}
	// All: expand every catalog version's learnset (picker order), tag Game, keep level order within each.
	vers, err := m.catalog.CatalogVersions()
	if err != nil || len(vers) == 0 {
		return r.Moves
	}
	byVG := map[int][]domain.LevelUpMove{}
	var out []domain.LevelUpMove
	for _, v := range vers {
		vg, ok := m.catalog.VersionGroupIDForVersion(v.ID)
		if !ok || vg < 1 {
			continue
		}
		rows, cached := byVG[vg]
		if !cached {
			rows, err = m.catalog.MovesForGroup(pokeID, vg)
			if err != nil || len(rows) == 0 {
				rows, err = m.catalog.LevelUpMovesForGroup(pokeID, vg)
			}
			if err != nil {
				rows = nil
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Level != rows[j].Level {
					return rows[i].Level < rows[j].Level
				}
				return rows[i].Move.DisplayName < rows[j].Move.DisplayName
			})
			byVG[vg] = rows
		}
		if len(rows) == 0 {
			continue
		}
		label := v.DisplayName
		if label == "" {
			label = v.Slug
		}
		for _, row := range rows {
			row.Game = label
			out = append(out, row)
		}
	}
	if len(out) == 0 {
		return r.Moves
	}
	return out
}

func (m *Model) dialogMoveRows() []domain.LevelUpMove {
	all := m.dialogMoveRowsAll()
	if len(all) == 0 {
		return nil
	}
	meth := m.dialogMoveMethod
	if meth == "" {
		meth = "level-up"
	}
	var out []domain.LevelUpMove
	for _, row := range all {
		rm := row.Method
		if rm == "" {
			rm = "level-up"
		}
		if rm == meth {
			out = append(out, row)
		}
	}
	return out
}
