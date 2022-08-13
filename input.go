package main;

import (
	"bufio"
	"os"
)

var scanner = bufio.NewScanner(os.Stdin)

func scanLine() string {
	if !scanner.Scan() {
		logFatal("failed to scan line")
	}
	return scanner.Text()
}
