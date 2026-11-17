package app

// Cell pixel-size probe (XTWINOPS CSI 16 t). Started as soon as native caps
// are known so it overlaps browse index/wiring; result is applied before
// tea.Run / warmers. Sixel paints at pixel resolution: encoding with a guessed
// cell height taller than the terminal's real cell spills past the art box.
// No reply within the timeout keeps the 10×20 default.

import (
	"bytes"
	"os"
	"time"

	"github.com/muesli/cancelreader"
	"golang.org/x/term"

	"github.com/Graphixa/Poke/internal/debuglog"
)

const cellSizeQueryTimeout = 50 * time.Millisecond

type cellSizeResult struct {
	pxW, pxH int
	ok       bool
}

// startQueryCellPixelSize runs the XTWINOPS probe off the critical path so
// browse wiring can overlap the wait. Caller must receive from the channel
// before tea.Run (probe owns raw stdin until done).
func startQueryCellPixelSize() <-chan cellSizeResult {
	ch := make(chan cellSizeResult, 1)
	go func() {
		w, h, ok := queryCellPixelSize()
		ch <- cellSizeResult{pxW: w, pxH: h, ok: ok}
	}()
	return ch
}

// queryCellPixelSize asks the terminal for its cell size in pixels.
// Safe no-op (ok=false) when stdin/stdout are not terminals or the host
// never replies.
func queryCellPixelSize() (pxW, pxH int, ok bool) {
	in, out := os.Stdin, os.Stdout
	if !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return 0, 0, false
	}
	oldState, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		return 0, 0, false
	}
	defer func() { _ = term.Restore(int(in.Fd()), oldState) }()

	cr, err := cancelreader.NewReader(in)
	if err != nil {
		return 0, 0, false
	}

	if _, err := out.WriteString("\x1b[16t"); err != nil {
		_ = cr.Close()
		return 0, 0, false
	}

	type result struct{ buf []byte }
	done := make(chan result, 1)
	go func() {
		var acc []byte
		chunk := make([]byte, 64)
		for {
			n, err := cr.Read(chunk)
			if n > 0 {
				acc = append(acc, chunk[:n]...)
				if _, _, found := parseCellSizeReport(acc); found {
					break
				}
			}
			if err != nil {
				break
			}
		}
		done <- result{buf: acc}
	}()

	var buf []byte
	select {
	case r := <-done:
		buf = r.buf
	case <-time.After(cellSizeQueryTimeout):
		cr.Cancel()
		r := <-done // reader unblocks on Cancel; never leaks a stdin read
		buf = r.buf
	}
	_ = cr.Close()
	// Cancel can split a CSI reply (e.g. bubbletea DECRPM ?2027;Ps$y) across the
	// consumed buf and the OS console buffer — swallow the tail while still raw.
	_ = drainRawOnce(in, stdinDrainIdle)

	pxW, pxH, ok = parseCellSizeReport(buf)
	debuglog.Renderf("render.cellsize.query ok=%v pxW=%d pxH=%d bytes=%d", ok, pxW, pxH, len(buf))
	return pxW, pxH, ok
}

// parseCellSizeReport extracts a CSI 6 ; height ; width t reply from raw
// input bytes (other queued input may surround it).
func parseCellSizeReport(buf []byte) (pxW, pxH int, ok bool) {
	const prefix = "\x1b[6;"
	i := bytes.Index(buf, []byte(prefix))
	if i < 0 {
		return 0, 0, false
	}
	rest := buf[i+len(prefix):]
	end := bytes.IndexByte(rest, 't')
	if end < 0 {
		return 0, 0, false
	}
	parts := bytes.Split(rest[:end], []byte(";"))
	if len(parts) != 2 {
		return 0, 0, false
	}
	h := atoiBytes(parts[0])
	w := atoiBytes(parts[1])
	if h < 1 || w < 1 {
		return 0, 0, false
	}
	return w, h, true
}

func atoiBytes(b []byte) int {
	n := 0
	for _, c := range b {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	if len(b) == 0 {
		return -1
	}
	return n
}

const (
	// ponytail: short idle — empty buffer paid this once per drain; was 40/200ms.
	stdinDrainIdle    = 10 * time.Millisecond
	stdinDrainMaxWait = 40 * time.Millisecond
)

// drainPendingStdin discards console input still sitting in the OS buffer.
// Browse search is always focused; leftover keys from setup ([Y]/N), theme
// picker teardown, a split cell-size/DECRPM reply, or Bubble Tea's Kitty
// keyboard query (CSI ? flags u) otherwise leak into the shell / first paint
// (e.g. "[?1u", "027;3$y" from mode 2027).
func drainPendingStdin() {
	in := os.Stdin
	if !term.IsTerminal(int(in.Fd())) {
		return
	}
	oldState, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		return
	}
	defer func() { _ = term.Restore(int(in.Fd()), oldState) }()

	deadline := time.Now().Add(stdinDrainMaxWait)
	total := 0
	for time.Now().Before(deadline) {
		n := drainRawOnce(in, stdinDrainIdle)
		total += n
		if n == 0 {
			break
		}
	}
	if total > 0 {
		debuglog.Renderf("stdin.drain discarded=%d", total)
	}
}

// drainRawOnce reads stdin until idle or Cancel. Caller must already be raw.
func drainRawOnce(in *os.File, idle time.Duration) int {
	cr, err := cancelreader.NewReader(in)
	if err != nil {
		return 0
	}
	defer func() { _ = cr.Close() }()

	done := make(chan int, 1)
	go func() {
		n := 0
		chunk := make([]byte, 64)
		for {
			r, err := cr.Read(chunk)
			n += r
			if err != nil || r == 0 {
				break
			}
		}
		done <- n
	}()

	select {
	case n := <-done:
		return n
	case <-time.After(idle):
		cr.Cancel()
		return <-done
	}
}
