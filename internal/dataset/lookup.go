package dataset

import (
	"sort"
	"strconv"

	"github.com/Graphixa/Poke/internal/domain"
)

// VersionGroupIDForVersion maps a version id to its group using the catalog.
func VersionGroupIDForVersion(versions map[int]DatasetVersion, versionID int) (int, bool) {
	v, ok := versions[versionID]
	if !ok || v.VersionGroupID < 1 {
		return 0, false
	}
	return v.VersionGroupID, true
}

// VersionBySlug finds a catalog version by slug.
func VersionBySlug(versions map[int]DatasetVersion, slug string) (DatasetVersion, bool) {
	for _, v := range versions {
		if v.Slug == slug {
			return v, true
		}
	}
	return DatasetVersion{}, false
}

// VersionsChronological returns catalog versions sorted by Order, then ID.
func VersionsChronological(versions map[int]DatasetVersion) []DatasetVersion {
	out := make([]DatasetVersion, 0, len(versions))
	for _, v := range versions {
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// NewestVersionWithCoverage returns the chronologically newest version that
// appears in any of the coverage slug/id sets. Coverage means imported rows
// exist — not that the game is complete. ok=false when none match.
func NewestVersionWithCoverage(versions map[int]DatasetVersion, versionIDs map[int]struct{}, versionSlugs map[string]struct{}) (DatasetVersion, bool) {
	ordered := VersionsChronological(versions)
	for i := len(ordered) - 1; i >= 0; i-- {
		v := ordered[i]
		if _, ok := versionIDs[v.ID]; ok {
			return v, true
		}
		if _, ok := versionSlugs[v.Slug]; ok {
			return v, true
		}
	}
	return DatasetVersion{}, false
}

// EncountersForVersion filters by exact version slug (or id when slug empty).
func EncountersForVersion(rows []DatasetEncounter, versionSlug string, versionID int) []DatasetEncounter {
	var out []DatasetEncounter
	for _, e := range rows {
		if versionID > 0 && e.VersionID == versionID {
			out = append(out, e)
			continue
		}
		if versionSlug != "" && e.Version == versionSlug {
			out = append(out, e)
		}
	}
	return out
}

// FlavorForVersion returns English flavour texts for an exact version.
func FlavorForVersion(texts []DatasetFlavorText, versionSlug string, versionID int) []DatasetFlavorText {
	var out []DatasetFlavorText
	for _, f := range texts {
		if versionID > 0 && f.VersionID == versionID {
			out = append(out, f)
			continue
		}
		if versionSlug != "" && f.Version == versionSlug {
			out = append(out, f)
		}
	}
	return out
}

// SelectableForms builds dossier form choices from species varieties + forms.
// The species default variety is labelled "Default" (relationship-based, not list position).
// Alternatives use short readable names (form_name / slug suffix), not "Species Alolan".
// Cosmetic non-default forms that share a Pokémon are appended when they have a form name.
func SelectableForms(sp DatasetSpecies, forms map[int]DatasetForm, pokemon map[int]DatasetPokemon) []domain.SelectableForm {
	var out []domain.SelectableForm
	seenPokemon := map[int]struct{}{}
	for _, v := range sp.Varieties {
		if v.PokemonID < 1 {
			continue
		}
		var name string
		if v.IsDefault {
			name = "Default"
		} else {
			pName := ""
			if p, ok := pokemon[v.PokemonID]; ok {
				pName = p.DisplayName
			}
			name = shortFormLabel(sp.Slug, sp.DisplayName, v.Slug, "", pName)
		}
		out = append(out, domain.SelectableForm{
			Key:         "pokemon:" + strconv.Itoa(v.PokemonID),
			PokemonID:   v.PokemonID,
			Slug:        v.Slug,
			DisplayName: name,
			IsDefault:   v.IsDefault,
		})
		seenPokemon[v.PokemonID] = struct{}{}
	}
	if len(out) == 0 && sp.DefaultPokemonID > 0 {
		out = append(out, domain.SelectableForm{
			Key:         "pokemon:" + strconv.Itoa(sp.DefaultPokemonID),
			PokemonID:   sp.DefaultPokemonID,
			Slug:        sp.Slug,
			DisplayName: "Default",
			IsDefault:   true,
		})
		seenPokemon[sp.DefaultPokemonID] = struct{}{}
	}

	type formRow struct {
		f DatasetForm
	}
	var extras []formRow
	for _, f := range forms {
		if _, ok := seenPokemon[f.PokemonID]; !ok {
			continue
		}
		if f.IsDefault || f.FormName == "" {
			continue
		}
		extras = append(extras, formRow{f: f})
	}
	sort.SliceStable(extras, func(i, j int) bool {
		if extras[i].f.FormOrder != extras[j].f.FormOrder {
			return extras[i].f.FormOrder < extras[j].f.FormOrder
		}
		return extras[i].f.ID < extras[j].f.ID
	})
	for _, row := range extras {
		f := row.f
		name := shortFormLabel(sp.Slug, sp.DisplayName, f.Slug, f.FormName, f.DisplayName)
		out = append(out, domain.SelectableForm{
			Key:         "form:" + strconv.Itoa(f.ID),
			PokemonID:   f.PokemonID,
			FormID:      f.ID,
			Slug:        f.Slug,
			DisplayName: name,
			IsDefault:   false,
		})
	}
	return out
}

func displayFromSlug(slug string) string {
	if slug == "" {
		return ""
	}
	// Keep dependency-free; pokeapi.DisplayNameFromSlug is richer but dataset must stay lean.
	out := make([]byte, 0, len(slug))
	capNext := true
	for i := 0; i < len(slug); i++ {
		c := slug[i]
		if c == '-' || c == '_' {
			out = append(out, ' ')
			capNext = true
			continue
		}
		if capNext && c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		capNext = false
		out = append(out, c)
	}
	return string(out)
}
