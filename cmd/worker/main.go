package main

import (
	"fmt"
	"os"

	_ "time/tzdata"

	"github.com/lunyashon/sdmp/internal/app"
)

func main() {
	if err := app.RunWorker(); err != nil {
		fmt.Fprintf(os.Stderr, "worker stopped with error: %v\n", err)
		os.Exit(1)
	}
}
