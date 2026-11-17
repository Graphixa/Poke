// Package dataset defines the aggregate snapshot layout and compact persisted models.
package dataset

// SchemaVersion is the dataset snapshot layout version (aggregate files, no per-record envelopes).
const SchemaVersion = 4

// Aggregate filenames inside datasets/<dataset-version>/.
const (
	FileManifest      = "manifest.json"
	FileBootstrap     = "bootstrap.json"
	FileBrowse        = "browse.json"
	FilePokemon       = "pokemon.json"
	FileSpecies       = "species.json"
	FileLearnsets     = "learnsets.json"
	FileMoves         = "moves.json"
	FileEvolutions    = "evolutions.json"
	FileEncounters    = "encounters.json"
	FileVersions      = "versions.json"
	FileVersionGroups = "version_groups.json"
	FileTypes         = "types.json"
	FileAbilities     = "abilities.json"
	FileForms         = "forms.json"
)

// Layout is the on-disk snapshot structure:
//
//	datasets/
//	  <dataset-version>/
//	    manifest.json
//	    bootstrap.json
//	    browse.json
//	    pokemon.json
//	    species.json
//	    learnsets.json
//	    moves.json
//	    evolutions.json
//	    encounters.json
//	    versions.json
//	    version_groups.json
//	    types.json
//	    abilities.json
//	    forms.json
type Layout struct{}
