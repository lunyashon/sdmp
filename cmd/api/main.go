package main

import (
	"fmt"
	"os"

	_ "time/tzdata"

	"github.com/lunyashon/sdmp/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "application stopped with error: %v\n", err)
		os.Exit(1)
	}
}
