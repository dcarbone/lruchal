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
	flagSet             *flag.FlagSet
	flagPort            uint
	flagCacheSize       uint
	flagConnectionLimit uint
)

func server() error {
	if flagPort == 0 || flagPort > math.MaxUint16 {
		return fmt.Errorf("port must be: 0 < port <= %d", math.MaxUint16)
	}
	if flagCacheSize == 0 || flagCacheSize > math.MaxInt64 {
		return fmt.Errorf("cachesize must be: 0 < cachesize <= %d", math.MaxInt64)
	}
	if flagConnectionLimit == 0 || flagConnectionLimit > math.MaxUint16 {
		return fmt.Errorf("connlimit must be: 0 < connlimit <= %d", math.MaxUint16)
	}

	config := &lruchal.ServerConfig{
		Port:            int(flagPort),
		CacheSize:       int(flagCacheSize),
		ConnectionLimit: int(flagConnectionLimit),
	}
	srv, err := lruchal.NewServer(config)
	if err != nil {
		return err
	}

	log.Printf("Using cache size: %d", flagCacheSize)
	log.Printf("Limiting concurrent connections to %d", flagConnectionLimit)
	log.Printf("Listening on port %d", flagPort)

	return srv.Serve()
}

func main() {
	flagSet = flag.NewFlagSet("lrutest", flag.ContinueOnError)
	flagSet.UintVar(&flagPort, "port", lruchal.DefaultPort, "Port to listen on")
	flagSet.UintVar(&flagCacheSize, "cachesize", lruchal.DefaultCacheSize, "Size of LRU cache")
	flagSet.UintVar(&flagConnectionLimit, "connlimit", lruchal.DefaultConnectionLimit, "Max allowable concurrent connections")
	flagSet.Parse(os.Args[1:])

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)

	go func() {
		errChan <- server()
	}()

	select {
	case err := <-errChan:
		if nil != err {
			log.Printf("Exiting with error: %s\n", err)
			os.Exit(1)
		}
		log.Println("Exiting")
	case sig := <-sigChan:
		log.Printf("\nSignal %s caught, exiting\n", sig)
	}

	os.Exit(0)
}
