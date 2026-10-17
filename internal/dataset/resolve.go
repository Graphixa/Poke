package dataset

import (
	"fmt"
	"strings"

	"github.com/Graphixa/Pokedex/internal/domain"
)

// ResolveInput is the caller's current dossier selection before resolution.
type ResolveInput struct {
	Species       DatasetSpecies
	PokemonID     int // 0 = use species default variety
	FormID        int // 0 = variety-level choice (no cosmetic form)
	GameVersionID int // 0 = unset game
	Versions      map[int]DatasetVersion
	VersionGroups map[int]DatasetVersionGroup
	Forms         map[int]DatasetForm
	Pokemon       map[int]DatasetPokemon
}

// ResolveDossierContext applies default-form identity, form×game compatibility,
// and automatic fallback when the choice is confirmed unsupported.
//
// Compatibility evidence (v1):
//   - Species default choice → Supported (fallback target; never auto-replaced).
//   - Intro version-group order strictly after the game's version-group order → Unsupported.
//   - Missing intro metadata, missing game, or game at/after intro → Unknown
//     (introduction does not prove availability in later games).
//   - Empty learnsets/encounters are never used as evidence.
func ResolveDossierContext(in ResolveInput) domain.ResolvedDossierContext {
	choices := SelectableForms(in.Species, in.Forms, in.Pokemon)
	def := defaultChoice(choices, in.Species)
	sel := findChoice(choices, in.PokemonID, in.FormID, def)

	out := domain.ResolvedDossierContext{
		SpeciesID:     in.Species.ID,
		PokemonID:     sel.PokemonID,
		FormID:        sel.FormID,
		GameVersionID: in.GameVersionID,
		FormLabel:     sel.DisplayName,
		Compat:        domain.FormCompatSupported,
	}
	if vg, ok := VersionGroupIDForVersion(in.Versions, in.GameVersionID); ok {
		out.VersionGroupID = vg
	}

	if in.GameVersionID < 1 {
		out.Compat = domain.FormCompatUnknown
		if sel.IsDefault {
			out.Compat = domain.FormCompatSupported
		}
		return out
	}

	out.Compat = ClassifyFormGame(sel, in.GameVersionID, in.Versions, in.VersionGroups, in.Forms)
	if out.Compat != domain.FormCompatUnsupported || sel.IsDefault {
		return out
	}

	// Confirmed unsupported → atomic switch to species default.
	gameLabel := gameDisplayName(in.Versions, in.GameVersionID)
	out.PokemonID = def.PokemonID
	out.FormID = def.FormID
	out.FormLabel = def.DisplayName
	out.Compat = domain.FormCompatSupported
	out.FellBackToDefault = true
	out.Notice = fmt.Sprintf("Switched to Default: the selected form is unavailable in %s.", gameLabel)
	return out
}

// ClassifyFormGame returns v1 compatibility for one form choice and game.
func ClassifyFormGame(
	choice domain.SelectableForm,
	gameVersionID int,
	versions map[int]DatasetVersion,
	versionGroups map[int]DatasetVersionGroup,
	forms map[int]DatasetForm,
) domain.FormGameCompat {
	if choice.IsDefault {
		return domain.FormCompatSupported
	}
	if gameVersionID < 1 {
		return domain.FormCompatUnknown
	}
	introVG := introVersionGroupID(choice, forms)
	if introVG < 1 {
		return domain.FormCompatUnknown
	}
	gameVG, ok := VersionGroupIDForVersion(versions, gameVersionID)
	if !ok {
		return domain.FormCompatUnknown
	}
	introOrder := versionGroupOrder(versionGroups, introVG)
	gameOrder := versionGroupOrder(versionGroups, gameVG)
	if introOrder < 1 || gameOrder < 1 {
		return domain.FormCompatUnknown
	}
	if gameOrder < introOrder {
		return domain.FormCompatUnsupported
	}
	// At or after introduction ≠ proven available.
	return domain.FormCompatUnknown
}

// FormOptionsForGame lists picker choices with per-game compatibility.
// Confirmed unsupported options remain listed so the UI can disable them.
func FormOptionsForGame(in ResolveInput) []domain.FormOption {
	choices := SelectableForms(in.Species, in.Forms, in.Pokemon)
	out := make([]domain.FormOption, 0, len(choices))
	for _, c := range choices {
		compat := domain.FormCompatSupported
		if in.GameVersionID > 0 {
			compat = ClassifyFormGame(c, in.GameVersionID, in.Versions, in.VersionGroups, in.Forms)
		} else if !c.IsDefault {
			compat = domain.FormCompatUnknown
		}
		out = append(out, domain.FormOption{Form: c, Compat: compat})
	}
	return out
}

// LearnsetPokemonID is the Pokémon identity used for learnsets/encounters.
// Cosmetic forms (FormID set) still use their parent PokemonID — never substitute
// the species default merely because a learnset slice is empty.
func LearnsetPokemonID(choice domain.SelectableForm) int {
	return choice.PokemonID
}

// LearnMethodsForGroup returns distinct learn methods present for a VG, stable order.
func LearnMethodsForGroup(rows []DatasetLearn, versionGroupID int) []string {
	filtered := FilterLearnset(rows, versionGroupID, "")
	if len(filtered) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var methods []string
	for _, row := range filtered {
		m := row.LearnMethod
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		methods = append(methods, m)
	}
	order := []string{"level-up", "machine", "egg", "tutor"}
	var out []string
	for _, m := range order {
		if _, ok := seen[m]; ok {
			out = append(out, m)
			delete(seen, m)
		}
	}
	for _, m := range methods {
		if _, ok := seen[m]; ok {
			out = append(out, m)
		}
	}
	return out
}

func introVersionGroupID(choice domain.SelectableForm, forms map[int]DatasetForm) int {
	if choice.FormID > 0 {
		if f, ok := forms[choice.FormID]; ok && f.IntroVersionGroupID > 0 {
			return f.IntroVersionGroupID
		}
	}
	// Variety-level: use the Pokémon's default form intro when present.
	var best DatasetForm
	found := false
	for _, f := range forms {
		if f.PokemonID != choice.PokemonID || f.IntroVersionGroupID < 1 {
			continue
		}
		if f.IsDefault {
			return f.IntroVersionGroupID
		}
		if !found || f.FormOrder < best.FormOrder || (f.FormOrder == best.FormOrder && f.ID < best.ID) {
			best = f
			found = true
		}
	}
	if found {
		return best.IntroVersionGroupID
	}
	return 0
}

func versionGroupOrder(groups map[int]DatasetVersionGroup, id int) int {
	if g, ok := groups[id]; ok {
		return g.Order
	}
	return 0
}

func defaultChoice(choices []domain.SelectableForm, sp DatasetSpecies) domain.SelectableForm {
	for _, c := range choices {
		if c.IsDefault {
			return c
		}
	}
	if sp.DefaultPokemonID > 0 {
		return domain.SelectableForm{
			Key:         fmt.Sprintf("pokemon:%d", sp.DefaultPokemonID),
			PokemonID:   sp.DefaultPokemonID,
			DisplayName: "Default",
			IsDefault:   true,
			Slug:        sp.Slug,
		}
	}
	if len(choices) > 0 {
		return choices[0]
	}
	return domain.SelectableForm{DisplayName: "Default", IsDefault: true}
}

func findChoice(choices []domain.SelectableForm, pokemonID, formID int, def domain.SelectableForm) domain.SelectableForm {
	if pokemonID < 1 && formID < 1 {
		return def
	}
	for _, c := range choices {
		if formID > 0 {
			if c.FormID == formID {
				return c
			}
			continue
		}
		if c.PokemonID == pokemonID && c.FormID == 0 {
			return c
		}
	}
	// Exact form/variety missing from list → treat as unknown selection; fall back to default identity for safety.
	return def
}

func gameDisplayName(versions map[int]DatasetVersion, id int) string {
	if v, ok := versions[id]; ok {
		if v.DisplayName != "" {
			return v.DisplayName
		}
		if v.Slug != "" {
			return displayFromSlug(v.Slug)
		}
	}
	return fmt.Sprintf("game #%d", id)
}

// shortFormLabel builds a user-facing alternative name without repeating the species.
func shortFormLabel(speciesSlug, speciesName, slug, formName, displayName string) string {
	if formName != "" {
		return displayFromSlug(formName)
	}
	if speciesSlug != "" {
		prefix := speciesSlug + "-"
		if strings.HasPrefix(slug, prefix) {
			rest := strings.TrimPrefix(slug, prefix)
			if rest != "" {
				return displayFromSlug(rest)
			}
		}
	}
	if displayName != "" && speciesName != "" && !strings.EqualFold(displayName, speciesName) {
		trimmed := strings.TrimSpace(strings.TrimPrefix(displayName, speciesName))
		trimmed = strings.TrimLeft(trimmed, " -")
		if trimmed != "" {
			return trimmed
		}
		return displayName
	}
	if displayName != "" {
		return displayName
	}
	return displayFromSlug(slug)
}
