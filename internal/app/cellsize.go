package app

// Cell pixel-size probe (XTWINOPS CSI 16 t), run once before the browse TUI
// starts. Sixel paints at pixel resolution: encoding with a guessed cell
// height taller than the terminal's real cell spills the sprite past the art
// box (browse overpaint). The reply (CSI 6 ; height ; width t) removes the
// guess; no reply within the timeout keeps the 10×20 default.

import (
	"bytes"
	"os"
	"time"

	"github.com/muesli/cancelreader"
	"golang.org/x/term"

	"github.com/Graphixa/Pokedex/internal/debuglog"
)

const cellSizeQueryTimeout = 200 * time.Millisecond

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
	defer func() { _ = cr.Close() }()

	if _, err := out.WriteString("\x1b[16t"); err != nil {
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

const stdinDrainTimeout = 30 * time.Millisecond

// drainPendingStdin discards console input still sitting in the OS buffer.
// Browse search is always focused; leftover keys from setup ([Y]/N) or the
// cell-size probe gap otherwise type themselves into the filter on first paint.
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

	cr, err := cancelreader.NewReader(in)
	if err != nil {
		return
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

	var n int
	select {
	case n = <-done:
	case <-time.After(stdinDrainTimeout):
		cr.Cancel()
		n = <-done
	}
	if n > 0 {
		debuglog.Renderf("stdin.drain discarded=%d", n)
	}
}
