package main

import (
	"fmt"
	"os"

	_ "time/tzdata"

	"github.com/lunyashon/sdmp/internal/app"
)

// @title          SDMP API
// @version        0.1
// @description    REST и приём трафика SDMP. Успешные ответы: {success:true, data:...}.
// @BasePath       /

//go:generate sh -c "cd ../.. && swag init -g cmd/api/main.go -o docs --parseInternal --exclude ./internal/adapter/http/crmdocs && swag init -g spec.go -d ./internal/adapter/http/crmdocs --instanceName crm -o docs --parseInternal --parseDependency"
func main() {
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "application stopped with error: %v\n", err)
		os.Exit(1)
	}
}
