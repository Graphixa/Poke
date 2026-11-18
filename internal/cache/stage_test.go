package cache

import "testing"

func TestDatasetStageWireValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		got  DatasetStage
		want string
	}{
		{DatasetStageIndex, "index"},
		{DatasetStageEnrich, "enrich"},
		{DatasetStageRecords, "records"},
		{DatasetStageEvolution, "evolution"},
		{DatasetStageMoves, "moves"},
		{DatasetStageDataset, "dataset"},
		{DatasetStageImport, "import"},
	}
	for _, tc := range cases {
		if string(tc.got) != tc.want {
			t.Fatalf("%q != %q", tc.got, tc.want)
		}
	}
}
