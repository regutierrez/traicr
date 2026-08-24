// Command traicr collects and uploads AI coding traces.
package main

import (
	"fmt"
	"os"

	"github.com/regutierrez/traicr/internal/version"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Printf("traicr %s\n", version.CurrentBuildInfo())
		return
	}

	fmt.Fprintln(os.Stderr, "usage: traicr version")
	os.Exit(2)
}
