// Adapted from https://github.com/erik-adelbert/firework (MIT)
// Dense single-width sparks so radial trails read as starburst spokes.

package launcher

import (
	"slices"

	"github.com/Graphixa/Poke/internal/fireworks/helper"
	"github.com/Graphixa/Poke/internal/fireworks/particle"
)

type runset struct {
	thresh float64
	runes  []rune
}

var palettes = [][]runset{
	particle.Spawning: {
		{0.3, []rune("·.+*")},
		{0.5, []rune("*+o")},
		{0.7, []rune("*oO@")},
		{1.0, []rune("@#%*")},
	},

	particle.Active: {
		{0.2, []rune("*+")},
		{0.5, []rune("*oO")},
		{0.8, []rune("@#*")},
		{1.0, []rune("@#%*")},
	},

	particle.Fading: {
		{0.5, []rune("·.+*")},
		{1.0, []rune(":+· ")},
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
