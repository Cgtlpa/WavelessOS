package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		die("usage: doit wave <acquire(install)|annihilate(remove)> <package-name>")
	}

	switch os.Args[1] {
	case "wave":
		wave()
	default:
		die("unknown command: %s", os.Args[1])
	}
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

func fatalln(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
