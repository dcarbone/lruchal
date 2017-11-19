package main

import (
	"flag"
	"fmt"
	"github.com/dcarbone/lruchal"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
)

var (
	flagSet *flag.FlagSet

	flagREPL       bool
	flagPort       uint
	flagConcurrent uint
	flagTTL        string
	flagSleep      string
)

func client() error {
	if flagPort == 0 || flagPort > math.MaxUint16 {
		return fmt.Errorf("port must be: 0 < port <= %d", math.MaxUint16)
	}

	return nil
}

func main() {
	flagSet = flag.NewFlagSet("client", flag.ContinueOnError)
	flagSet.BoolVar(&flagREPL, "repl", false, "Start in interactive mode")
	flagSet.UintVar(&flagPort, "port", lruchal.DefaultPort, "Port to listen on")
	flagSet.UintVar(&flagConcurrent, "concurrent", 10, "Concurrent client count")
	flagSet.StringVar(&flagTTL, "ttl", "1m", "TTL per concurrent client")
	flagSet.StringVar(&flagSleep, "sleep", "500ms", "Time per client to sleep between actions")
	flagSet.Parse(os.Args[1:])

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- client()
	}()

	select {
	case err := <-errChan:
		if nil != err {
			log.Printf("Exiting with error: %s\n", err)
			os.Exit(1)
		}
		log.Println("Exiting")
	case sig := <-sigChan:
		log.Printf("Signal %s caught, exiting\n", sig)
	}

	os.Exit(0)
}
