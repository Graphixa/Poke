package views

import "github.com/Graphixa/Pokedex/internal/render/sprite"

// Browse glance (View 1) uses a stats-matched sprite bucket. View 2 (hero art)
// uses MaxFrameSize. Leftover vertical space stays empty under the tile.
const (
	GlanceBucket = 16 // ArtCardH matches StatsCardOuter height
	HeroBucket   = sprite.MaxFrameSize

	glanceHeaderRows = 2
	glanceCardBorder = 2
	glanceContentPad = 1 // row/col of space inside art and stats cards
	glanceCardGap    = 1
	glanceMinAboutH  = 3 // 1 content row + borders

	// Stats bar line is "~8 label + 1 + 12 bar + 2 + 3 value" ≈ 26 cells.
	glanceStatsContentW = 26
	glanceStatsContentH = 8 // 6 bars + sep + total

	// Abilities: two content rows (slots 1–2 / slot 3) after pad; used for hug height.
	glanceAboutContentH = 2
)

// GlanceLayout is the C3 right-pane geometry for a fixed terminal size.
type GlanceLayout struct {
	Bucket     int
	ShowSprite bool
	ShowStats  bool
	ShowAbout  bool

	ArtCardX, ArtCardY int // origin within body (hero may centre)
	ArtCardW, ArtCardH int
	ArtW, ArtH         int
	ArtPadX, ArtPadY   int

	StatsX, StatsW, StatsH int
	AboutY, AboutH         int
	UpperH                 int
}

// ArtCardOuter returns the fixed outer size for the browse art card at bucket.
// Includes border + glanceContentPad on each inner edge.
func ArtCardOuter(bucket int) (w, h int) {
	aw, ah := sprite.FrameSizeFor(bucket)
	return aw + glanceCardBorder + 2*glanceContentPad, ah + glanceCardBorder + 2*glanceContentPad
}

// StatsCardOuter returns the fixed outer size for the browse Stats card.
// Height hugs stats content (no stretch to match the art card).
func StatsCardOuter() (w, h int) {
	w = glanceStatsContentW + glanceCardBorder + 2*glanceContentPad
	h = glanceStatsContentH + glanceCardBorder + 2*glanceContentPad
	return w, h
}

// aboutCardHugH is the outer height for an About card with contentLines of
// real text (before GlanceStatsBody pad), not stretched to fill the pane.
func aboutCardHugH(contentLines int) int {
	if contentLines < 1 {
		contentLines = 1
	}
	h := contentLines + 2*glanceContentPad + glanceCardBorder
	if h < glanceMinAboutH {
		h = glanceMinAboutH
	}
	return h
}

// GlanceRightInnerWidth is the fixed right-pane content width for art+gap+stats
// (About spans this width). Layout gives leftover terminal columns to the list.
func GlanceRightInnerWidth() int {
	artW, _ := ArtCardOuter(GlanceBucket)
	statsW, _ := StatsCardOuter()
	return artW + glanceCardGap + statsW
}

// HeroRightInnerWidth is the right-pane content width for View 2 (art only).
func HeroRightInnerWidth() int {
	w, _ := ArtCardOuter(HeroBucket)
	return w
}

// ComputeGlance lays out the fixed art+stats tile (View 1). About hugs content
// when wantAbout; leftover body height stays empty under the tile.
func ComputeGlance(rightInnerW, rightInnerH int, wantAbout bool) GlanceLayout {
	bodyH := rightInnerH - glanceHeaderRows
	if bodyH < 1 {
		bodyH = 1
	}
	if rightInnerW < 1 {
		return GlanceLayout{}
	}

	bucket := GlanceBucket
	artW, artH := sprite.FrameSizeFor(bucket)
	cardW, cardH := ArtCardOuter(bucket)
	statsW, statsH := StatsCardOuter()

	needW := cardW + glanceCardGap + statsW
	if cardW <= rightInnerW && cardH <= bodyH {
		showStats := rightInnerW >= needW
		sw, sh, sx := 0, 0, 0
		upperH := cardH
		if showStats {
			sx = cardW + glanceCardGap
			sw, sh = statsW, statsH
			if sh > upperH {
				upperH = sh
			}
		}

		showAbout := false
		aboutH := 0
		aboutY := upperH
		if wantAbout {
			// Only show when the full hug fits — clamping below hug yields an
			// empty titled box (pad rows eat the single content line).
			hug := aboutCardHugH(glanceAboutContentH)
			if bodyH-upperH >= hug {
				showAbout = true
				aboutH = hug
			}
		}

		return GlanceLayout{
			Bucket:     bucket,
			ShowSprite: true,
			ShowStats:  showStats,
			ShowAbout:  showAbout,
			ArtCardW:   cardW,
			ArtCardH:   cardH,
			ArtW:       artW,
			ArtH:       artH,
			ArtPadX:    glanceContentPad,
			ArtPadY:    glanceContentPad,
			StatsX:     sx,
			StatsW:     sw,
			StatsH:     sh,
			AboutY:     aboutY,
			AboutH:     aboutH,
			UpperH:     upperH,
		}
	}

	return statsOnlyGlance(rightInnerW, bodyH, wantAbout)
}

// ComputeHeroArt lays out View 2: largest art card only, centred in the body.
func ComputeHeroArt(rightInnerW, rightInnerH int) GlanceLayout {
	bodyH := rightInnerH - glanceHeaderRows
	if bodyH < 1 {
		bodyH = 1
	}
	if rightInnerW < 1 {
		return GlanceLayout{}
	}

	bucket := HeroBucket
	artW, artH := sprite.FrameSizeFor(bucket)
	cardW, cardH := ArtCardOuter(bucket)
	if cardW > rightInnerW || cardH > bodyH {
		// Step down until it fits; never below glance bucket.
		for _, side := range sprite.FrameStepsMaxFirst() {
			if side < GlanceBucket {
				continue
			}
			cw, ch := ArtCardOuter(side)
			if cw <= rightInnerW && ch <= bodyH {
				bucket = side
				artW, artH = sprite.FrameSizeFor(side)
				cardW, cardH = cw, ch
				break
			}
			bucket = 0
		}
		if bucket < GlanceBucket {
			return GlanceLayout{}
		}
	}

	artX := (rightInnerW - cardW) / 2
	if artX < 0 {
		artX = 0
	}
	artY := (bodyH - cardH) / 2
	if artY < 0 {
		artY = 0
	}

	return GlanceLayout{
		Bucket:     bucket,
		ShowSprite: true,
		ShowStats:  false,
		ShowAbout:  false,
		ArtCardX:   artX,
		ArtCardY:   artY,
		ArtCardW:   cardW,
		ArtCardH:   cardH,
		ArtW:       artW,
		ArtH:       artH,
		ArtPadX:    glanceContentPad,
		ArtPadY:    glanceContentPad,
		UpperH:     cardH,
	}
}

func statsOnlyGlance(rightInnerW, bodyH int, wantAbout bool) GlanceLayout {
	statsW, statsH := StatsCardOuter()
	if statsW > rightInnerW {
		statsW = rightInnerW
	}
	if statsW < glanceStatsContentW+glanceCardBorder || bodyH < glanceStatsContentH+glanceCardBorder {
		return GlanceLayout{}
	}
	if statsH > bodyH {
		statsH = bodyH
	}

	showAbout := false
	aboutH := 0
	if wantAbout {
		hug := aboutCardHugH(glanceAboutContentH)
		if bodyH-statsH >= hug {
			showAbout = true
			aboutH = hug
		}
	}

	return GlanceLayout{
		ShowSprite: false,
		ShowStats:  true,
		ShowAbout:  showAbout,
		StatsX:     0,
		StatsW:     statsW, // still fixed, not full pane
		StatsH:     statsH,
		AboutY:     statsH,
		AboutH:     aboutH,
		UpperH:     statsH,
	}
}

// PreferredBucket is the glance bucket when the art card fits (View 1).
func PreferredBucket(rightInnerW, rightInnerH int) int {
	g := ComputeGlance(rightInnerW, rightInnerH, false)
	if g.ShowSprite {
		return g.Bucket
	}
	return 0
}

// PreferredHeroBucket is the hero art bucket when it fits (View 2).
func PreferredHeroBucket(rightInnerW, rightInnerH int) int {
	g := ComputeHeroArt(rightInnerW, rightInnerH)
	if g.ShowSprite {
		return g.Bucket
	}
	return 0
}

// GlanceStatsBody pads stats lines with one blank row/col inside the card.
func GlanceStatsBody(lines []string) []string {
	pad := stringsRepeatSpaces(glanceContentPad)
	out := make([]string, 0, len(lines)+2*glanceContentPad)
	for i := 0; i < glanceContentPad; i++ {
		out = append(out, "")
	}
	for _, line := range lines {
		out = append(out, pad+line)
	}
	for i := 0; i < glanceContentPad; i++ {
		out = append(out, "")
	}
	return out
}

func stringsRepeatSpaces(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
