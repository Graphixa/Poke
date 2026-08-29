//go:build rendertest

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func registerTestCommand(parent *cobra.Command, options Options) {
	parent.AddCommand(newTestCommand(options))
}

func newTestCommand(options Options) *cobra.Command {
	var iters int
	var benchOnly bool
	cmd := &cobra.Command{
		Use:   "test [pokemon]",
		Short: "Compare chafa vs halfblock (imgcat-style) vs termimg at three sizes",
		Long:  "Developer diagnostic: visual side-by-side + timed drag race for sprite renderers. Default subject is Pikachu (#25). Rebuild with -tags rendertest.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.RenderTest == nil {
				return fmt.Errorf("poke test is unavailable")
			}
			target := ""
			if len(args) == 1 {
				target = args[0]
			}
			return options.RenderTest.RunRenderTest(cmd.Context(), RenderTestRequest{
				Target:     target,
				Iterations: iters,
				SkipVisual: benchOnly,
			})
		},
	}
	cmd.Flags().IntVar(&iters, "iters", 30, "timed iterations per backend/size")
	cmd.Flags().BoolVar(&benchOnly, "bench", false, "timings only (skip visual dumps)")
	return cmd
}
