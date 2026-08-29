//go:build !rendertest

package app

import (
	"context"
	"fmt"
)

func (h *Handler) RunRenderTest(_ context.Context, _ RenderTestRequest) error {
	return fmt.Errorf("poke test is a developer diagnostic; rebuild with -tags rendertest")
}
