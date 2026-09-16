package output

import "testing"

func TestFormatGenerationRoman(t *testing.T) {
	cases := map[string]string{
		"generation-i":  "Generation I",
		"generation-iv": "Generation IV",
		"generation-ix": "Generation IX",
		"GENERATION-II": "Generation II",
		"":              "",
		"gen-1":         "",
	}
	for in, want := range cases {
		if got := FormatGeneration(in); got != want {
			t.Fatalf("FormatGeneration(%q)=%q want %q", in, got, want)
		}
	}
	if got := GenerationNumeral("generation-iv"); got != "IV" {
		t.Fatalf("GenerationNumeral = %q want IV", got)
	}
}
