package cache

import "testing"

func TestStageWireValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		got  Stage
		want string
	}{
		{StageIndex, "index"},
		{StageEnrich, "enrich"},
		{StageRecords, "records"},
		{StageEvolution, "evolution"},
		{StageMoves, "moves"},
		{StageDataset, "dataset"},
		{StageArtwork, "artwork"},
		{StageNative, "native"},
		{StageSpritesRender, "sprites_render"},
		{StageDossier, "dossier"},
		{StageSprites, "sprites"},
	}
	for _, tc := range cases {
		if string(tc.got) != tc.want {
			t.Fatalf("%q != %q", tc.got, tc.want)
		}
	}
}
