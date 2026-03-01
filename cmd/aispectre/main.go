package main

import (
	"fmt"
	"os"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("aispectre %s (commit: %s, built: %s)\n", version, commit, date)
		return
	}
	fmt.Println("aispectre — AI/LLM spend waste auditor")
	os.Exit(0)
}
