package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"parkops/internal/platform/security"
)

func main() {
	password := flag.String("password", "", "password to hash")
	flag.Parse()

	if strings.TrimSpace(*password) == "" {
		fmt.Fprintln(os.Stderr, "usage: hashgen -password <value>")
		os.Exit(2)
	}

	hash, err := security.HashPassword(*password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error hashing password: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(hash)
}
