package views_test

import (
	"strings"
	"testing"

	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/render/sprite"
	"github.com/Graphixa/Poke/internal/tui/views"
)

func TestGlanceBucketMatchesStatsHeight(t *testing.T) {
	g := views.ComputeGlance(80, 50, true)
	if g.Bucket != 16 {
		t.Fatalf("bucket=%d want 16", g.Bucket)
	}
	if !g.ShowSprite || !g.ShowStats || !g.ShowAbout {
		t.Fatalf("sprite=%v stats=%v about=%v", g.ShowSprite, g.ShowStats, g.ShowAbout)
	}
	cw, ch := views.ArtCardOuter(16)
	if g.ArtCardW != cw || g.ArtCardH != ch {
		t.Fatalf("art card %dx%d want %dx%d", g.ArtCardW, g.ArtCardH, cw, ch)
	}
	_, statsH := views.StatsCardOuter()
	if g.StatsH != statsH {
		t.Fatalf("StatsH=%d want content hug %d", g.StatsH, statsH)
	}
	if g.ArtCardH != g.StatsH {
		t.Fatalf("View1 art outer must match stats outer: art=%d stats=%d", g.ArtCardH, g.StatsH)
	}
	if g.UpperH != g.ArtCardH {
		t.Fatalf("UpperH=%d want %d", g.UpperH, g.ArtCardH)
	}
	aw, ah := sprite.FrameSizeFor(16)
	if aw != 16 || ah != 8 {
		t.Fatalf("FrameSizeFor(16)=%dx%d want 16x8", aw, ah)
	}
}

func TestGlanceTallDoesNotGrowCards(t *testing.T) {
	short := views.ComputeGlance(80, 2+16+5, true)
	tall := views.ComputeGlance(80, 2+60, true)
	if short.ArtCardH != tall.ArtCardH || short.StatsH != tall.StatsH {
		t.Fatal("art/stats must not grow with terminal height")
	}
	if short.AboutH != tall.AboutH {
		t.Fatalf("About must hug content, not grow: short=%d tall=%d", short.AboutH, tall.AboutH)
	}
	if tall.AboutH > 4 {
		t.Fatalf("AboutH=%d looks stretched (want hug ~4)", tall.AboutH)
	}
	if _, statsH := views.StatsCardOuter(); tall.StatsH != statsH {
		t.Fatalf("StatsH=%d want %d", tall.StatsH, statsH)
	}
}

func TestGlanceFixedStatsWidth(t *testing.T) {
	g := views.ComputeGlance(80, 40, false)
	sw, _ := views.StatsCardOuter()
	if g.StatsW != sw {
		t.Fatalf("StatsW=%d want fixed %d (not pane remainder)", g.StatsW, sw)
	}
	if g.StatsW >= 80-g.ArtCardW {
		t.Fatal("stats must not stretch across the right pane")
	}
}

func TestPreferredBucket16OrZero(t *testing.T) {
	if b := views.PreferredBucket(80, 50); b != 16 {
		t.Fatalf("PreferredBucket=%d want 16", b)
	}
	_, artH := views.ArtCardOuter(views.GlanceBucket)
	if b := views.PreferredBucket(80, 2+(artH-1)); b != 0 {
		t.Fatalf("too short PreferredBucket=%d want 0", b)
	}
}

func TestGlanceRightPanelStable(t *testing.T) {
	w := views.GlanceRightInnerWidth() + 1
	if w < 40 || w > 80 {
		t.Fatalf("GlanceRightPanelWidth=%d looks wrong", w)
	}
}

func TestBrowseGlanceShowsStatsAndAbilities(t *testing.T) {
	rw := views.GlanceRightInnerWidth() + 1
	out := views.Browse(views.BrowseData{
		Width: 120, Height: 52,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 39, DisplayName: "Jigglypuff", Types: []string{"normal", "fairy"}},
		ShowSprite:  true, VerboseFooter: true,
		BodySize:  views.GlanceBucket,
		LeftWidth: 120 - rw - 1, RightWidth: rw,
		HeaderTypes:    "Normal/Fairy",
		HasBrowseStats: true,
		Stats:          domain.Stats{HP: 115, Attack: 45, Defense: 20, SpecialAttack: 45, SpecialDefense: 25, Speed: 20},
		HasCore:        true,
		Abilities: []domain.AbilitySlot{
			{Slot: 1, Name: "cute-charm"},
			{Slot: 2, Name: "competitive"},
			{Slot: 3, Name: "friend-guard", IsHidden: true},
		},
		SpriteANSI: "[sprite]",
		ImageState: 2,
	})
	for _, want := range []string{"Stats", "HP", "Abilities", "Cute Charm", "Jigglypuff", "#039"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestBrowseGlanceHidesAbilitiesWithoutCore(t *testing.T) {
	rw := views.GlanceRightInnerWidth() + 1
	out := views.Browse(views.BrowseData{
		Width: 120, Height: 52,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 39, DisplayName: "Jigglypuff"},
		ShowSprite:  true,
		BodySize:    views.GlanceBucket,
		LeftWidth:   120 - rw - 1, RightWidth: rw,
		HasBrowseStats: true,
		Stats:          domain.Stats{HP: 115},
		HasCore:        false,
		SpriteANSI:     "[sprite]",
		ImageState:     2,
	})
	if strings.Contains(out, " Abilities ") {
		t.Fatalf("Abilities should be hidden without HasCore:\n%s", out)
	}
}

func TestBrowseGlanceCardsHugOnTallTerminal(t *testing.T) {
	rw := views.GlanceRightInnerWidth() + 1
	g := views.ComputeGlance(rw-1, 80-1-2, true) // right inner ≈ panelH-2 with Height 80
	_, wantStatsH := views.StatsCardOuter()
	if g.StatsH != wantStatsH {
		t.Fatalf("StatsH=%d want hug %d", g.StatsH, wantStatsH)
	}
	if g.AboutH < 3 || g.AboutH > 4 {
		t.Fatalf("AboutH=%d want hug 3..4", g.AboutH)
	}
	_, wantArtH := views.ArtCardOuter(views.GlanceBucket)
	if g.ArtCardH != wantArtH {
		t.Fatalf("ArtCardH=%d want %d", g.ArtCardH, wantArtH)
	}

	out := views.Browse(views.BrowseData{
		Width: 120, Height: 80,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 2, DisplayName: "Ivysaur"},
		ShowSprite:  true,
		BodySize:    views.GlanceBucket,
		LeftWidth:   120 - rw - 1, RightWidth: rw,
		HasBrowseStats: true,
		Stats:          domain.Stats{HP: 60, Attack: 62, Defense: 63, SpecialAttack: 80, SpecialDefense: 80, Speed: 60},
		HasCore:        true,
		Abilities: []domain.AbilitySlot{
			{Slot: 1, Name: "overgrow"},
			{Slot: 2, Name: "chlorophyll"},
		},
		SpriteANSI: "[sprite]",
		ImageState: 2,
	})
	if !strings.Contains(out, "Abilities") || !strings.Contains(out, "Stats") {
		t.Fatalf("missing cards:\n%s", out)
	}
	// Tall pane must not leave About as a giant bordered void: hug AboutH from layout.
	tall := views.ComputeGlance(rw-1, 80-1-2, true)
	short := views.ComputeGlance(rw-1, 2+16+5, true)
	if tall.AboutH != short.AboutH {
		t.Fatalf("rendered layout About grew: tall=%d short=%d", tall.AboutH, short.AboutH)
	}
}

func TestSpriteArtOriginStable(t *testing.T) {
	rw := views.GlanceRightInnerWidth() + 1
	base := views.BrowseData{
		Width: 120, Height: 52,
		ShowSprite: true, SpriteNative: true,
		LeftWidth: 120 - rw - 1, RightWidth: rw,
		BodySize: views.GlanceBucket, HasSelected: true,
		Selected: domain.SpeciesSummary{ID: 39, DisplayName: "Jigglypuff"},
	}
	r1, c1, w1, h1, ok := views.SpriteArtOrigin(base)
	if !ok {
		t.Fatal("expected origin")
	}
	if w1 != 16 || h1 != 8 {
		t.Fatalf("art %dx%d want 16x8", w1, h1)
	}
	base.HasCore = true
	base.Height = 80
	r2, c2, w2, h2, ok := views.SpriteArtOrigin(base)
	if !ok || r1 != r2 || c1 != c2 || w1 != w2 || h1 != h2 {
		t.Fatalf("origin moved: %d,%d %dx%d vs %d,%d %dx%d", r1, c1, w1, h1, r2, c2, w2, h2)
	}
}

// TestSpriteArtOriginMatchesRenderedCard locks paint CUP to the Glance art
// inner box (not the card border / not a bottom strip).
func TestSpriteArtOriginMatchesRenderedCard(t *testing.T) {
	rw := views.GlanceRightInnerWidth() + 1
	d := views.BrowseData{
		Width: 120, Height: 50,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 25, DisplayName: "Pikachu"},
		ShowSprite:  true, SpriteNative: true,
		BodySize:  views.GlanceBucket,
		LeftWidth: 120 - rw - 1, RightWidth: rw,
		HasBrowseStats: true,
		Stats:          domain.Stats{HP: 35, Attack: 55, Defense: 40, SpecialAttack: 50, SpecialDefense: 50, Speed: 90},
		HasCore:        true,
		Abilities:      []domain.AbilitySlot{{Slot: 1, Name: "blaze"}},
		HeaderTypes:    "Electric",
		ImageState:     2,
	}
	row, col, artW, artH, ok := views.SpriteArtOrigin(d)
	if !ok {
		t.Fatal("expected origin")
	}
	if artW != 16 || artH != 8 {
		t.Fatalf("art %dx%d want 16x8", artW, artH)
	}

	plain := stripANSI(views.Browse(d))
	lines := strings.Split(plain, "\n")
	artTop, artCol := -1, -1
	for i, line := range lines {
		runes := []rune(line)
		for j := 0; j < len(runes)-1; j++ {
			if runes[j] != '╭' || runes[j+1] != '─' || j == 0 || runes[j-1] != '│' {
				continue
			}
			rest := string(runes[j:])
			if strings.HasPrefix(rest, "╭─ Stats") || strings.HasPrefix(rest, "╭─ Abilities") {
				continue
			}
			artTop, artCol = i, j
			break
		}
		if artTop >= 0 {
			break
		}
	}
	if artTop < 0 {
		t.Fatalf("art card not found:\n%s", plain)
	}
	g := views.ComputeGlance(rw-1, 50-1-2, true)
	wantRow := artTop + 1 + 1 + g.ArtPadY // 1-index border row + inner top + pad
	wantCol := artCol + 1 + 1 + g.ArtPadX
	if row != wantRow || col != wantCol {
		t.Fatalf("SpriteArtOrigin=%d,%d want card-inner %d,%d (card top 0-index %d,%d pad %d,%d)",
			row, col, wantRow, wantCol, artTop, artCol, g.ArtPadX, g.ArtPadY)
	}
}

func TestGlanceAboutRequiresFullHug(t *testing.T) {
	_, artH := views.ArtCardOuter(views.GlanceBucket)
	const hug = 4 // glanceAboutContentH(2) + borders, pad=0
	// rightInnerH = header + art + (hug-1) → remain too small for full About hug.
	tooTight := views.ComputeGlance(80, 2+artH+(hug-1), true)
	if tooTight.ShowAbout {
		t.Fatalf("About must not show when remain < hug (got AboutH=%d)", tooTight.AboutH)
	}
	fits := views.ComputeGlance(80, 2+artH+hug, true)
	if !fits.ShowAbout || fits.AboutH != hug {
		t.Fatalf("Abilities should hug at %d when it fits: show=%v h=%d", hug, fits.ShowAbout, fits.AboutH)
	}
}

func TestBrowseGlanceAbilitiesVisibleMediumHeight(t *testing.T) {
	rw := views.GlanceRightInnerWidth() + 1
	_, artH := views.ArtCardOuter(views.GlanceBucket)
	// Terminal tall enough for header+art+about (+frame/footer chrome).
	termH := 2 /*frame*/ + 2 /*header*/ + artH + 6 + 1 /*footer*/ + 2
	out := views.Browse(views.BrowseData{
		Width: 120, Height: termH,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 1, DisplayName: "Bulbasaur"},
		ShowSprite:  true,
		BodySize:    views.GlanceBucket,
		LeftWidth:   120 - rw - 1, RightWidth: rw,
		HeaderTypes: "Grass/Poison", HeaderGenus: "Seed Pokémon",
		HasBrowseStats: true,
		Stats:          domain.Stats{HP: 45, Attack: 49, Defense: 49, SpecialAttack: 65, SpecialDefense: 65, Speed: 45},
		HasCore:        true,
		Abilities: []domain.AbilitySlot{
			{Slot: 1, Name: "overgrow"},
			{Slot: 2, Name: "chlorophyll"},
		},
		SpriteANSI: "[sprite]",
		ImageState: 2,
	})
	plain := stripANSI(out)
	if !strings.Contains(plain, "Abilities") || !strings.Contains(plain, "Overgrow") {
		t.Fatalf("Abilities missing at medium height:\n%s", plain)
	}
}

func TestBrowseArtCardPinnedSize(t *testing.T) {
	rw := views.GlanceRightInnerWidth() + 1
	cw, ch := views.ArtCardOuter(views.GlanceBucket)
	out := views.Browse(views.BrowseData{
		Width: 120, Height: 50,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 1, DisplayName: "Bulbasaur"},
		ShowSprite:  true,
		BodySize:    views.GlanceBucket,
		LeftWidth:   120 - rw - 1, RightWidth: rw,
		HasBrowseStats: true,
		Stats:          domain.Stats{HP: 45},
		HasCore:        true,
		Abilities:      []domain.AbilitySlot{{Slot: 1, Name: "overgrow"}},
		SpriteANSI:     "X",
		ImageState:     2,
	})
	plain := stripANSI(out)
	lines := strings.Split(plain, "\n")
	artTop := -1
	artCol := -1
	for i, line := range lines {
		if strings.Contains(line, "Pokedex") {
			continue
		}
		runes := []rune(line)
		for j := 0; j < len(runes)-1; j++ {
			// Untitled art: │╭─… not │╭─ Stats / │╭─ Abilities.
			if runes[j] != '╭' || runes[j+1] != '─' || j == 0 || runes[j-1] != '│' {
				continue
			}
			rest := string(runes[j:])
			if strings.HasPrefix(rest, "╭─ Stats") || strings.HasPrefix(rest, "╭─ Abilities") {
				continue
			}
			artTop = i
			artCol = j
			break
		}
		if artTop >= 0 {
			break
		}
	}
	if artTop < 0 || artCol < 0 {
		t.Fatalf("art card top not found:\n%s", plain)
	}
	artBottom := -1
	for i := artTop + 1; i < len(lines) && i <= artTop+ch; i++ {
		runes := []rune(lines[i])
		if artCol < len(runes) && runes[artCol] == '╰' {
			artBottom = i
			break
		}
	}
	if artBottom < 0 {
		t.Fatalf("art card bottom not found at col %d after line %d:\n%s", artCol, artTop, plain)
	}
	gotH := artBottom - artTop + 1
	if gotH != ch {
		t.Fatalf("art card height=%d want pinned %d (outer %dx%d)", gotH, ch, cw, ch)
	}
	g := views.ComputeGlance(rw-1, 50-1-2, true)
	if g.ArtW != 16 || g.ArtH != 8 || g.ArtPadX != 0 || g.ArtPadY != 0 {
		t.Fatalf("art inner %dx%d pad %d,%d want 16x8 pad 0", g.ArtW, g.ArtH, g.ArtPadX, g.ArtPadY)
	}
	if g.ArtCardH != ch || g.StatsH != g.ArtCardH {
		t.Fatalf("ArtCardH=%d StatsH=%d want equal outer heights", g.ArtCardH, g.StatsH)
	}
}

func TestBrowseArtViewNoStatsAbilities(t *testing.T) {
	rw := views.HeroRightInnerWidth() + 1
	out := views.Browse(views.BrowseData{
		Width: 140, Height: 52,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 25, DisplayName: "Pikachu"},
		ShowSprite:  true,
		BrowseView:  1,
		BodySize:    views.HeroBucket,
		LeftWidth:   140 - rw - 1, RightWidth: rw,
		HasCore:        true,
		Abilities:      []domain.AbilitySlot{{Slot: 1, Name: "blaze"}},
		HasBrowseStats: true,
		Stats:          domain.Stats{HP: 35},
		SpriteANSI:     "[hero]",
		ImageState:     2,
	})
	plain := stripANSI(out)
	if strings.Contains(plain, "Stats") || strings.Contains(plain, "Abilities") {
		t.Fatalf("Art view must omit Stats/Abilities:\n%s", plain)
	}
	if !strings.Contains(out, "[hero]") {
		t.Fatalf("missing hero sprite:\n%s", out)
	}
	g := views.ComputeHeroArt(rw-1, 52-1-2)
	if g.Bucket != views.HeroBucket || g.ShowStats || g.ShowAbout {
		t.Fatalf("hero layout bucket=%d stats=%v about=%v", g.Bucket, g.ShowStats, g.ShowAbout)
	}
	if g.ArtW != 40 || g.ArtH != 20 {
		t.Fatalf("hero art %dx%d want 40x20", g.ArtW, g.ArtH)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestArtCardCentresSprite(t *testing.T) {
	// Smoke: browse with ANSI sprite includes the payload (centering is structural).
	rw := views.GlanceRightInnerWidth() + 1
	out := views.Browse(views.BrowseData{
		Width: 100, Height: 40,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 39, DisplayName: "Jigglypuff"},
		ShowSprite:  true,
		BodySize:    sprite.MinFrameSize,
		LeftWidth:   100 - rw - 1, RightWidth: rw,
		SpriteANSI: "SPRITEROW",
		ImageState: 2,
	})
	if !strings.Contains(out, "SPRITEROW") {
		t.Fatalf("missing sprite in:\n%s", out)
	}
}
