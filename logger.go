package lruchal

import (
	"log"
	"os"
)

type Logger interface {
	Print(...interface{})
	Printf(string, ...interface{})
}

type DefaultLoggerFunc func(prefix string) Logger

var DefaultLogger DefaultLoggerFunc = func(prefix string) Logger {
	return log.New(os.Stdout, prefix, log.LstdFlags)
}
