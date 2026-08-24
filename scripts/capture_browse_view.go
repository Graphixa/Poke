//go:build ignore

// Captures a live browse TUI frame matching the tui-example.md scenario.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/app"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/halfblock"
	"github.com/Graphixa/Pokedex/internal/render/sprites"
	"github.com/Graphixa/Pokedex/internal/search"
	"github.com/Graphixa/Pokedex/internal/tui"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	paths, err := config.OSDirectoryResolver().Resolve()
	if err != nil {
		fatal(err)
	}
	svc, err := app.LoadServices(paths)
	if err != nil {
		fatal(err)
	}
	indexRows, _, err := svc.Cache.EnsureIndex(ctx, nil)
	if err != nil {
		fatal(err)
	}
	idx := search.Build(indexRows)

	var selected *domain.SpeciesSummary
	for _, s := range indexRows {
		if s.ID == 4 || strings.EqualFold(s.Slug, "charmander") {
			cp := s
			selected = &cp
			break
		}
	}
	if selected == nil {
		fatal(fmt.Errorf("charmander not in index"))
	}

	ds := app.NewDetailService(svc.Cache)
	src := detailAdapter{DetailService: ds}
	renderer := &sprites.Renderer{
		Resolver:  &sprites.Resolver{Store: svc.Cache, API: svc.API},
		Artefacts: svc.Cache,
		Enabled:   svc.Config.Render.Halfblock,
		Palette:   halfblock.DetectPalette(),
	}

	m := tui.NewModel(tui.Options{
		Index:    idx,
		Details:  src,
		Sprites:  spriteAdapter{renderer},
		Selected: selected,
		Prefill:  "c",
		Context:  ctx,
	})

	var model tea.Model = m
	model, cmd := model.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	drain(&model, cmd)
	drain(&model, model.(tui.Model).Init())

	// Wait until main info has loaded (Ability / genus / height).
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		plain := stripANSI(model.(tui.Model).View().Content)
		if strings.Contains(plain, "Ability:") || strings.Contains(plain, "Height:") || strings.Contains(plain, "Lizard") {
			break
		}
		time.Sleep(150 * time.Millisecond)
		drain(&model, model.(tui.Model).Init())
	}

	// Stats -> Evolution
	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	drain(&model, cmd)

	deadline = time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		plain := stripANSI(model.(tui.Model).View().Content)
		if strings.Contains(plain, "Charmeleon") || strings.Contains(plain, "Level 16") {
			break
		}
		time.Sleep(150 * time.Millisecond)
		drain(&model, model.(tui.Model).Init())
	}

	raw := model.(tui.Model).View().Content
	plain := stripANSI(raw)

	outDir := filepath.Join("scripts", "captures")
	_ = os.MkdirAll(outDir, 0o755)
	_ = os.WriteFile(filepath.Join(outDir, "browse-charmander.ansi.txt"), []byte(raw), 0o644)
	_ = os.WriteFile(filepath.Join(outDir, "browse-charmander.txt"), []byte(plain), 0o644)
	fmt.Fprintln(os.Stderr, "captured scripts/captures/browse-charmander.txt")
	fmt.Println(plain)
}

type detailAdapter struct {
	*app.DetailService
}

func (d detailAdapter) Load(ctx context.Context, speciesID int) (domain.Pokemon, domain.Species, error) {
	res, err := d.DetailService.Load(ctx, speciesID)
	if err != nil {
		return domain.Pokemon{}, domain.Species{}, err
	}
	return res.Pokemon, res.Species, nil
}

func (d detailAdapter) LevelUpMoves(ctx context.Context, pokemon domain.Pokemon) ([]tui.LevelUpMove, error) {
	rows, err := d.DetailService.LevelUpMoves(ctx, pokemon)
	if err != nil {
		return nil, err
	}
	out := make([]tui.LevelUpMove, len(rows))
	for i, row := range rows {
		out[i] = tui.LevelUpMove{Level: row.Level, Move: row.Move}
	}
	return out, nil
}

type spriteAdapter struct {
	*sprites.Renderer
}

func (s spriteAdapter) Render(ctx context.Context, pokemon domain.Pokemon, cells int) (string, bool) {
	if s.Renderer == nil {
		return "", true
	}
	res := s.Renderer.Render(ctx, pokemon, cells)
	return res.ANSI, res.UsePlaceholder
}

func drain(model *tea.Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	switch msg := msg.(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			drain(model, c)
		}
	case nil:
		return
	default:
		next, nextCmd := (*model).Update(msg)
		*model = next
		drain(model, nextCmd)
	}
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*(?:\x07|\x1b\\)|\x1b.`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
