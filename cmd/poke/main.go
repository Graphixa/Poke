package main

import (
	"context"
	"os"

	"github.com/Graphixa/Poke/internal/app"
)

func main() {
	os.Exit(app.Main(context.Background(), app.OSRuntime()))
}
