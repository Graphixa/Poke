// Package dataset defines the v3 aggregate snapshot layout and compact persisted models.
package dataset

// SchemaVersion is the dataset snapshot layout version (aggregate files, no per-record envelopes).
const SchemaVersion = 3

// Aggregate filenames inside datasets/<dataset-version>/.
const (
	FileManifest   = "manifest.json"
	FileBootstrap  = "bootstrap.json"
	FileBrowse     = "browse.json"
	FilePokemon    = "pokemon.json"
	FileSpecies    = "species.json"
	FileLearnsets  = "learnsets.json"
	FileMoves      = "moves.json"
	FileEvolutions = "evolutions.json"
	FileEncounters = "encounters.json"
)

// Layout is the v3 on-disk snapshot structure:
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
//
// Each aggregate file is compact JSON (no indentation). Dataset-wide metadata lives
// in manifest.json; individual records are not wrapped in per-file envelopes.
type Layout struct{}
