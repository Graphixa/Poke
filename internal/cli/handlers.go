package cli

import (
	"context"
)

// AppHandler runs product commands delegated to the application layer.
type AppHandler interface {
	RunRoot(context.Context, RootRequest) error
	Search(context.Context, string) error
	Random(context.Context, InlineMode) error
	Theme(context.Context) error
}
