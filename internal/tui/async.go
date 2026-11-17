package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Poke/internal/domain"
)

type dossierCoreMsg struct {
	gen       int
	speciesID int
	slug      string
	dossier   domain.Dossier
	complete  bool
	err       error
}

type dossierExtrasMsg struct {
	gen       int
	speciesID int
	dossier   domain.Dossier
	err       error
}

type dossierMsg struct {
	gen       int
	speciesID int
	dossier   domain.Dossier
	err       error
}

type spriteMsg struct {
	reqID       int
	gen         int
	speciesID   int
	cells       int
	ansi        string
	placeholder bool
	native      bool
	assetKind   string
}

// warmHintMinInterval throttles footer "warming N/M" updates to reduce Sixel wipes.
const warmHintMinInterval = 250 * time.Millisecond

func loadDossierCoreCmd(ctx context.Context, src DetailSource, gen, speciesID int, slug string) tea.Cmd {
	if src == nil {
		return nil
	}
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return dossierCoreMsg{gen: gen, speciesID: speciesID, slug: slug, err: err}
		}
		d, complete, err := src.LoadDossierCore(ctx, speciesID, slug)
		return dossierCoreMsg{gen: gen, speciesID: speciesID, slug: slug, dossier: d, complete: complete, err: err}
	}
}

func loadDossierExtrasCmd(ctx context.Context, src DetailSource, gen, speciesID int, slug string, core domain.Dossier) tea.Cmd {
	if src == nil {
		return nil
	}
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return dossierExtrasMsg{gen: gen, speciesID: speciesID, err: err}
		}
		d, err := src.LoadDossierExtras(ctx, speciesID, slug, core)
		return dossierExtrasMsg{gen: gen, speciesID: speciesID, dossier: d, err: err}
	}
}

type warmProgressMsg struct {
	Done      int
	Total     int
	Message   string
	SpeciesID int
	Stage     WarmStage
	Cells     int
	closed    bool
}

func listenWarmProgress(ch <-chan WarmProgress) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return warmProgressMsg{closed: true}
		}
		return warmProgressMsg{
			Done: p.Done, Total: p.Total, Message: p.Message,
			SpeciesID: p.SpeciesID, Stage: p.Stage, Cells: p.Cells,
		}
	}
}
