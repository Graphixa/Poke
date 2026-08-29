package main

import (
	"context"
	"os"

	"github.com/Graphixa/Pokedex/internal/app"
)

func main() {
	os.Exit(app.Main(context.Background(), app.OSRuntime()))
}
