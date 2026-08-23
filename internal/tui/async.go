package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
)

type coreMsg struct {
	gen       int
	speciesID int
	pokemon   domain.Pokemon
	species   domain.Species
	err       error
}

type evolutionMsg struct {
	gen       int
	speciesID int
	tree      domain.EvolutionTree
	err       error
}

type encountersMsg struct {
	gen       int
	speciesID int
	rows      []domain.Encounter
	err       error
}

type movesMsg struct {
	gen       int
	speciesID int
	rows      []LevelUpMove
	err       error
}

type spriteMsg struct {
	reqID       int
	gen         int
	speciesID   int
	cells       int
	ansi        string
	placeholder bool
}

func loadCoreCmd(ctx context.Context, src DetailSource, gen, speciesID int) tea.Cmd {
	if src == nil {
		return nil
	}
	return func() tea.Msg {
		pokemon, species, err := src.Load(ctx, speciesID)
		return coreMsg{gen: gen, speciesID: speciesID, pokemon: pokemon, species: species, err: err}
	}
}

func loadEvolutionCmd(ctx context.Context, src DetailSource, gen, speciesID int, species domain.Species) tea.Cmd {
	if src == nil {
		return nil
	}
	return func() tea.Msg {
		tree, err := src.Evolution(ctx, species)
		return evolutionMsg{gen: gen, speciesID: speciesID, tree: tree, err: err}
	}
}

func loadEncountersCmd(ctx context.Context, src DetailSource, gen, speciesID, pokemonID int) tea.Cmd {
	if src == nil {
		return nil
	}
	return func() tea.Msg {
		rows, err := src.Encounters(ctx, pokemonID)
		return encountersMsg{gen: gen, speciesID: speciesID, rows: rows, err: err}
	}
}

func loadMovesCmd(ctx context.Context, src DetailSource, gen, speciesID int, pokemon domain.Pokemon) tea.Cmd {
	if src == nil {
		return nil
	}
	return func() tea.Msg {
		rows, err := src.LevelUpMoves(ctx, pokemon)
		return movesMsg{gen: gen, speciesID: speciesID, rows: rows, err: err}
	}
}

func loadSpriteCmd(ctx context.Context, src SpriteRenderer, reqID, gen, speciesID, cells int, pokemon domain.Pokemon) tea.Cmd {
	if src == nil || cells < 1 {
		return func() tea.Msg {
			return spriteMsg{reqID: reqID, gen: gen, speciesID: speciesID, cells: cells, placeholder: true}
		}
	}
	return func() tea.Msg {
		ansi, placeholder := src.Render(ctx, pokemon, cells)
		return spriteMsg{
			reqID:       reqID,
			gen:         gen,
			speciesID:   speciesID,
			cells:       cells,
			ansi:        ansi,
			placeholder: placeholder || ansi == "",
		}
	}
}
