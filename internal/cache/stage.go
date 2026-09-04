package cache

// DatasetStage names a cache/dataset progress phase. Values are the historical
// wire strings used by CLI copy and the bootstrap TUI. Changing a constant
// value is a user-visible change. Native/artwork/dossier/chafa stages belong
// to the producing warmer packages, not this type.
type DatasetStage string

const (
	DatasetStageIndex     DatasetStage = "index"
	DatasetStageEnrich    DatasetStage = "enrich"
	DatasetStageRecords   DatasetStage = "records"
	DatasetStageEvolution DatasetStage = "evolution"
	DatasetStageMoves     DatasetStage = "moves"
	DatasetStageDataset   DatasetStage = "dataset"
)
