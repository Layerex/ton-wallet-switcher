package main

import (
	"fmt"
	"os"
)

func logMessage(prefix string, v ...interface{}) {
	fmt.Fprintln(os.Stderr, prefix, fmt.Sprint(v...))
}

func logInfo(v ...interface{}) {
	logMessage("Info:", v...)
}

func logError(v ...interface{}) {
	logMessage("Error:", v...)
}

func logFatal(v ...interface{}) {
	logError(v...)
	os.Exit(1)
}

func logHelp(v ...interface{}) {
	logError(v...)
	Help()
	os.Exit(1)
}
