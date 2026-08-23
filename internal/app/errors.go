package app

import "fmt"

// NotFoundError is returned for unknown Pokémon in inline mode.
type NotFoundError struct {
	Query string
}

func (e *NotFoundError) Error() string {
	if e.Query == "" {
		return "Pokémon not found"
	}
	return fmt.Sprintf("Pokémon not found: %s", e.Query)
}

func (e *NotFoundError) ExitCode() int { return 1 }
