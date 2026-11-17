package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/Graphixa/Poke/internal/domain"
)

// SearchTable writes a plain-text search result table.
func SearchTable(w io.Writer, results []domain.SpeciesSummary) error {
	if len(results) == 0 {
		_, err := fmt.Fprintln(w, "No Pokémon matched.")
		return err
	}
	nameWidth := len("Name")
	typeWidth := len("Types")
	for _, r := range results {
		if n := len(r.DisplayName); n > nameWidth {
			nameWidth = n
		}
		if t := JoinTypes(r.Types); len(t) > typeWidth {
			typeWidth = len(t)
		}
	}
	header := fmt.Sprintf("%-5s  %-*s  %-*s", "No.", nameWidth, "Name", typeWidth, "Types")
	if _, err := fmt.Fprintln(w, header); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, strings.Repeat("-", len(header))); err != nil {
		return err
	}
	for _, r := range results {
		types := JoinTypes(r.Types)
		if types == "" {
			types = "-"
		}
		line := fmt.Sprintf("%-5s  %-*s  %-*s", FormatDex(r.ID), nameWidth, r.DisplayName, typeWidth, types)
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}
