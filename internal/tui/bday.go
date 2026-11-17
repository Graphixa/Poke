package tui

import (
	"image/color"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Poke/internal/fireworks"
	"github.com/Graphixa/Poke/internal/fireworks/shells"
	"github.com/Graphixa/Poke/internal/fireworks/sym"
)

const birthdayTrigger = "declan"
const birthdayEnterNeed = 3

// Flip to true for theme-tinted fireworks (classic presets when false).
const birthdayUseThemeColors = true

type birthdayTickMsg time.Time

func birthdayTick() tea.Cmd {
	return tea.Tick(30*time.Millisecond, func(t time.Time) tea.Msg {
		return birthdayTickMsg(t)
	})
}

func isBirthdayQuery(q string) bool {
	return strings.EqualFold(strings.TrimSpace(q), birthdayTrigger)
}

func (m Model) handleBirthdayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.showBirthday = false
		return m, nil
	}
	return m, nil
}

func (m Model) handleBirthdayEnter() (tea.Model, tea.Cmd) {
	if !isBirthdayQuery(m.queryValue()) {
		m.declanEnterCount = 0
		return m.openDialog()
	}
	m.declanEnterCount++
	if m.declanEnterCount < birthdayEnterNeed {
		return m, nil
	}
	return m.openBirthday()
}

func (m Model) openBirthday() (tea.Model, tea.Cmd) {
	m.showBirthday = true
	m.declanEnterCount = 0
	shells.UseThemeColors = birthdayUseThemeColors
	now := time.Now()
	w, h := birthdayGrid(m.width, m.height)
	if m.birthday == nil {
		m.birthday = fireworks.NewShow(w, h, now)
	} else {
		m.birthday.Resize(w, h)
		m.birthday.Reset()
		m.birthday.Trigger(now)
	}
	m, cover := m.coverNativeOverlay()
	return m, tea.Batch(cover, birthdayTick())
}

func (m Model) onBirthdayTick(msg birthdayTickMsg) (tea.Model, tea.Cmd) {
	if !m.showBirthday || m.birthday == nil {
		return m, nil
	}
	m.birthday.Step(time.Time(msg))
	return m, birthdayTick()
}

func (m *Model) resizeBirthday() {
	if !m.showBirthday || m.birthday == nil {
		return
	}
	w, h := birthdayGrid(m.width, m.height)
	m.birthday.Resize(w, h)
}

func birthdayGrid(termW, termH int) (w, h int) {
	w, h = termW, termH
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

func (m Model) birthdayView() string {
	if m.birthday == nil {
		return ""
	}
	w, h := birthdayGrid(m.width, m.height)
	m.birthday.Resize(w, h)
	m.birthday.Paint()
	stampBirthdayBanner(m.birthday.Screen(), w, h)
	return m.birthday.Stringify()
}

// Solid block banner (spaces are transparent so fireworks show behind).
var birthdayBanner = []string{
	`█  █ ▄▀▀▄ █▀▀▄ █▀▀▄ ▀▄ ▄▀      █▀▀▄ █ █▀▀▄ ▀▀█▀▀ █  █ █▀▀▄ ▄▀▀▄ ▀▄ ▄▀`,
	`█▀▀█ █▀▀█ █▀▀  █▀▀    █        █▀▀▄ █ █▀▀▄   █   █▀▀█ █  █ █▀▀█   █  `,
	`▀  ▀ ▀  ▀ ▀    ▀      ▀        ▀▀▀  ▀ ▀  ▀   ▀   ▀  ▀ ▀▀▀  ▀  ▀   ▀  `,
	``,
	`█▀▀▄ █▀▀▀ ▄▀▀▄ █    ▄▀▀▄ █▄  █`,
	`█  █ █▀▀  █  ▄ █    █▀▀█ █ ▀▄█`,
	`▀▀▀  ▀▀▀▀  ▀▀  ▀▀▀▀ ▀  ▀ ▀   ▀`,
}

var bannerWhite = color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}

func stampBirthdayBanner(screen []sym.Symbol, w, h int) {
	if w < 1 || h < 1 || len(screen) < w*h {
		return
	}
	banner := birthdayBanner
	maxW := 0
	for _, ln := range banner {
		if n := len([]rune(ln)); n > maxW {
			maxW = n
		}
	}
	if w < maxW {
		banner = []string{"Happy Birthday", "Declan"}
	}

	startRow := (h - len(banner)) / 2
	if startRow < 0 {
		startRow = 0
	}

	for i, ln := range banner {
		row := startRow + i
		if row < 0 || row >= h {
			continue
		}
		runes := []rune(ln)
		// Center each line so Declan sits under Happy Birthday.
		col := (w - len(runes)) / 2
		if col < 0 {
			col = 0
		}
		for _, r := range runes {
			if col >= w {
				break
			}
			if !unicode.IsSpace(r) {
				screen[row*w+col] = sym.MkSymbol(r, bannerWhite)
			}
			col++
		}
	}
}
