package cache

import "testing"

func TestBootstrapReportReady(t *testing.T) {
	ok := DatasetBootstrapReport{
		Expected: 2, PokemonOK: 2, SpeciesOK: 2, BrowseOK: 2,
		EncounterOK: 2, EvolutionWanted: 1, EvolutionOK: 1,
		MoveWanted: 1, MoveOK: 1,
	}
	if !ok.Ready() {
		t.Fatal("complete report must be ready")
	}
	partial := ok
	partial.PokemonOK = 1
	if partial.Ready() {
		t.Fatal("missing pokemon must not be ready")
	}
	withFail := ok
	withFail.add("pokemon", 26, errNotReady, true)
	if withFail.Ready() {
		t.Fatal("required failure must not be ready")
	}
}

var errNotReady = errString("x")

type errString string

func (e errString) Error() string { return string(e) }
