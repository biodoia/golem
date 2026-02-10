package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/biodoia/golem/internal/cli"
	"github.com/biodoia/golem/internal/ui"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] != "" && !strings.HasPrefix(os.Args[1], "-") {
		query := strings.Join(os.Args[1:], " ")
		if err := cli.RunOneShot(query); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := ui.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
