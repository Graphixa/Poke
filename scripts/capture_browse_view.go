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
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	spritechafa "github.com/Graphixa/Pokedex/internal/render/sprite/chafa"
	spriteimage "github.com/Graphixa/Pokedex/internal/render/sprite/image"
	"github.com/Graphixa/Pokedex/internal/render/sprite/placeholder"
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
	img := spriteimage.New()
	caps := sprite.DetectCapabilities(img.Available())
	colorMode := sprite.ColorTrueColor
	if !caps.TrueColor {
		colorMode = sprite.Color256
	}
	engine := &sprite.Engine{
		Source: &sprite.SourceResolver{Store: svc.Cache, API: svc.API},
		Modes: sprite.NewModeResolver(
			img,
			spritechafa.Renderer{},
			placeholder.Renderer{},
		),
		Artefacts:     svc.Cache,
		Caps:          caps,
		Preference:    sprite.RenderMode(svc.Config.EffectiveSpriteMode()),
		Color:         colorMode,
		SourceKind:    sprite.SourcePixel,
		ClearNativeFn: spriteimage.ClearBestEffort,
		OnNativeFail:  img.Disable,
	}

	m := tui.NewModel(tui.Options{
		Index:              idx,
		Details:            src,
		Sprites:            spriteAdapter{engine},
		Selected:           selected,
		Prefill:            "c",
		Context:            ctx,
	})

	var model tea.Model = m
	model, cmd := model.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
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

func (d detailAdapter) LoadDossierCore(ctx context.Context, speciesID int, slug string) (domain.Dossier, bool, error) {
	return d.DetailService.LoadDossierCore(ctx, speciesID, slug)
}

func (d detailAdapter) LoadDossierExtras(ctx context.Context, speciesID int, slug string, core domain.Dossier) (domain.Dossier, error) {
	return d.DetailService.LoadDossierExtras(ctx, speciesID, slug, core)
}

func (d detailAdapter) LoadDossier(ctx context.Context, speciesID int) (domain.Dossier, error) {
	return d.DetailService.LoadDossier(ctx, speciesID)
}

type spriteAdapter struct {
	*sprite.Engine
}

func (s spriteAdapter) Render(ctx context.Context, pokemon domain.Pokemon, cells int) (string, bool, bool, string) {
	if s.Engine == nil {
		return "", true, false, ""
	}
	res := s.Engine.Render(ctx, pokemon, cells)
	return res.ANSI, res.UsePlaceholder, res.Native, res.AssetKind
}

func (s spriteAdapter) ClearNative() {
	if s.Engine != nil {
		s.Engine.ClearNative()
	}
}

func (s spriteAdapter) InvalidateSpecies(speciesID int) {
	if s.Engine != nil {
		s.Engine.InvalidateSpecies(speciesID)
	}
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
