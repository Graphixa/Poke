package output

import "testing"

func TestCleanFlavorJoinsSoftHyphenLineBreaks(t *testing.T) {
	// Ivysaur Gold (PokeAPI): soft hyphen + newline mid-"sunlight", form-feed mid-sentence.
	in := "Exposure to sun\u00ad\nlight adds to its\nstrength. Sunlight\falso makes the bud\non its back grow\nlarger."
	got := CleanFlavor(in)
	want := "Exposure to sunlight adds to its strength. Sunlight also makes the bud on its back grow larger."
	if got != want {
		t.Fatalf("CleanFlavor=\n%q\nwant\n%q", got, want)
	}
	if containsSoftHyphen(got) {
		t.Fatal("cleaned flavor still contains U+00AD soft hyphen")
	}
}

func containsSoftHyphen(s string) bool {
	for _, r := range s {
		if r == '\u00ad' {
			return true
		}
	}
	return false
}

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
