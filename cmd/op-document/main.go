package main

import (
	"os"

	"github.com/gloamers/openproject-bridge/internal/app/documentapp"
)

func main() {
	os.Exit(documentapp.Main(os.Args[1:]))
}
