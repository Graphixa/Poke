package cache

// Stage names a progress phase. Values are the historical wire strings used by
// CLI copy, the bootstrap TUI, and warmer → browse footer mapping. Changing a
// constant value is a user-visible change.
type Stage string

const (
	StageIndex         Stage = "index"
	StageEnrich        Stage = "enrich"
	StageRecords       Stage = "records"
	StageEvolution     Stage = "evolution"
	StageMoves         Stage = "moves"
	StageDataset       Stage = "dataset"
	StageArtwork       Stage = "artwork"
	StageNative        Stage = "native"
	StageSpritesRender Stage = "sprites_render"
	StageDossier       Stage = "dossier"
	StageSprites       Stage = "sprites"
)
