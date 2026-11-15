// Adapted from https://github.com/erik-adelbert/firework (MIT)
// Glyphs switched to small ASCII sparks for denser terminal fireworks.

package launcher

import (
	"slices"

	"github.com/Graphixa/Pokedex/internal/fireworks/helper"
	"github.com/Graphixa/Pokedex/internal/fireworks/particle"
)

type runset struct {
	thresh float64
	runes  []rune
}

var palettes = [][]runset{
	particle.Spawning: {
		{0.3, []rune("·.,'`")},
		{0.5, []rune(":+*")},
		{0.7, []rune("*oO")},
		{1.0, []rune("@#%")},
	},

	particle.Active: {
		{0.2, []rune("·.")},
		{0.6, []rune(":+*")},
		{0.85, []rune("*o")},
		{1.0, []rune("@#")},
	},

	particle.Fading: {
		{0.6, []rune("·.,'")},
		{1.0, []rune(":+ ")},
	},

	particle.Dead: {
		{1.0, []rune(" ")},
	},
}

func getrune(p *particle.Particle, d float64) rune {
	lut := palettes[p.State()]

	runes := lut[slices.IndexFunc(lut, func(rs runset) bool {
		return d <= rs.thresh
	})].runes

	return randomRune(runes)
}

func randomRune(runes []rune) rune {
	if len(runes) == 0 {
		return ' '
	}

	r, _ := helper.Pick(runes)

	return r
}
