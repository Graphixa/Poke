package cli

import (
	"context"
)

// SearchHandler runs poke search <query>.
type SearchHandler interface {
	Search(context.Context, string) error
}

// SearchHandlerFunc adapts a function to SearchHandler.
type SearchHandlerFunc func(context.Context, string) error

func (f SearchHandlerFunc) Search(ctx context.Context, query string) error {
	return f(ctx, query)
}

// ChafaHandlerFunc adapts a function to ChafaHandler.
type ChafaHandlerFunc func(context.Context) error

func (f ChafaHandlerFunc) TestChafaSprites(ctx context.Context) error {
	return f(ctx)
}

// RandomHandler runs poke random [--info|--stats|--evolution].
type RandomHandler interface {
	Random(context.Context, InlineMode) error
}

// RandomHandlerFunc adapts a function to RandomHandler.
type RandomHandlerFunc func(context.Context, InlineMode) error

func (f RandomHandlerFunc) Random(ctx context.Context, mode InlineMode) error {
	return f(ctx, mode)
}
