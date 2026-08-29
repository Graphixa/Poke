//go:build !rendertest

package cli

import "github.com/spf13/cobra"

func registerTestCommand(_ *cobra.Command, _ Options) {}
