package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/dcarbone/lruchal"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	flagSet *flag.FlagSet

	flagREPL       bool
	flagPort       uint
	flagConcurrent uint
	flagTTL        string
	flagSleep      string
)

func benchmark() error {
	ttl, err := time.ParseDuration(flagTTL)
	if err != nil {
		return fmt.Errorf("invalid ttl format: %s", err)
	}
	sleep, err := time.ParseDuration(flagSleep)
	if err != nil {
		return fmt.Errorf("invalid sleep format: %s", err)
	}
	if flagPort == 0 || flagPort > math.MaxUint16 {
		return fmt.Errorf("port must be: 0 < port <= %d", math.MaxUint16)
	}

	now := time.Now()
	requests := uint64(0)
	fails := uint64(0)
	wg := new(sync.WaitGroup)
	wg.Add(int(flagConcurrent))
	for i := uint(0); i < flagConcurrent; i++ {
		log.Printf("starting %d", i)
		go func(i int) {
			timer := time.NewTimer(ttl)
			defer func() {
				timer.Stop()
				wg.Done()
				log.Printf("ending %d", i)
			}()
			client, err := lruchal.NewClient(&lruchal.ClientConfig{
				HttpClient: &http.Client{
					Transport: &http.Transport{
						DisableKeepAlives: true,
						Proxy:             http.ProxyFromEnvironment,
						DialContext: (&net.Dialer{
							Timeout:   30 * time.Second,
							KeepAlive: 30 * time.Second,
						}).DialContext,
						MaxIdleConns:          100,
						IdleConnTimeout:       90 * time.Second,
						TLSHandshakeTimeout:   10 * time.Second,
						ExpectContinueTimeout: 1 * time.Second,
						MaxIdleConnsPerHost:   -1,
					},
				},
				Address: fmt.Sprintf(":%d", flagPort),
			})
			if err != nil {
				log.Printf("Unable to create client: %s", err)
				return
			}
			for i := 0; ; i++ {
				select {
				case <-timer.C:
					return
				default:
					atomic.AddUint64(&requests, 1)
					if i%2 == 0 {
						err := client.Put(lruchal.Item{strconv.Itoa(rand.Intn(i + 1)), fmt.Sprintf("value%d", i), "1s"})
						if err != nil {
							atomic.AddUint64(&fails, 1)
							log.Printf("put error: %s", err)
						} else {
							log.Println("put ok")
						}
					} else {
						v, err := client.Get(strconv.Itoa(rand.Intn(i + 1)))
						if err != nil {
							atomic.AddUint64(&fails, 1)
							log.Printf("get error: %s", err)
						} else {
							log.Printf("get value: %#v", v)
						}
					}
				}
				time.Sleep(sleep)
			}
		}(int(i))
	}
	wg.Wait()

	log.Printf("Completed %d requests with %d fails in %s", requests, fails, time.Now().Sub(now))

	return nil
}

func repl() error {
	client, err := lruchal.NewClient(&lruchal.ClientConfig{
		Address: fmt.Sprintf(":%d", flagPort),
	})
	if err != nil {
		return err
	}

	mu := new(sync.Mutex)
	fs := flag.NewFlagSet("repl", flag.ContinueOnError)
	keyPtr := fs.String("k", "", "Key to interact with")
	valuePtr := fs.String("v", "", "Value to put to key")
	ttlPtr := fs.String("ttl", "", "TTL to store with put key")

	stdinChan := make(chan string, 10)
	defer close(stdinChan)

	reader := bufio.NewReader(os.Stdin)
	errChan := make(chan error, 2)

	go func() {
		for {
			input, err := reader.ReadString('\n')
			if err != nil {
				errChan <- fmt.Errorf("unable to read from stdin: %s", err)
				return
			}
			stdinChan <- strings.TrimSpace(input)
		}
	}()

	go func() {
		for in := range stdinChan {
			mu.Lock()
			args := strings.Split(in, " ")
			if len(args) == 0 {
				fmt.Fprintln(os.Stdout, "command must be provided")
			} else {
				switch args[0] {
				case "get":
					if err := fs.Parse(args[1:]); err != nil {
						fmt.Fprintf(os.Stdout, "Parse error: %s\n", err)
					} else {
						v, err := client.Get(*keyPtr)
						if err != nil {
							fmt.Fprintf(os.Stdout, "Error: %s\n", err)
						} else {
							fmt.Fprintf(os.Stdout, "%#v\n", v)
						}
					}
				case "put":
					if err := fs.Parse(args[1:]); err != nil {
						fmt.Fprintf(os.Stdout, "Parse error: %s\n", err)
					} else if *ttlPtr == "" {
						fmt.Fprintln(os.Stdout, "ttl flag cannot be empty")
					} else if _, err := time.ParseDuration(*ttlPtr); err != nil {
						fmt.Fprintf(os.Stdout, "invalid ttl format specified: %s\n", err)
					} else {
						err := client.Put(lruchal.Item{*keyPtr, *valuePtr, *ttlPtr})
						if err != nil {
							fmt.Fprintf(os.Stdout, "Error: %s\n", err)
						} else {
							fmt.Fprintln(os.Stdout, "OK")
						}
					}
				default:
					fmt.Fprintf(os.Stdout, "unknown command \"%s\"", args[0])
				}
			}
			mu.Unlock()
		}
	}()

	return <-errChan
}

func main() {
	rand.Seed(time.Now().UnixNano())

	flagSet = flag.NewFlagSet("client", flag.ContinueOnError)
	flagSet.BoolVar(&flagREPL, "repl", false, "Start in interactive mode")
	flagSet.UintVar(&flagPort, "port", lruchal.DefaultPort, "Port to listen on")
	flagSet.UintVar(&flagConcurrent, "concurrent", 10, "Concurrent client count")
	flagSet.StringVar(&flagTTL, "ttl", "1m", "TTL per concurrent client")
	flagSet.StringVar(&flagSleep, "sleep", "500ms", "Time per client to sleep between actions")
	flagSet.Parse(os.Args[1:])

	if flagPort == 0 || flagPort > math.MaxUint16 {
		log.Printf("port must be: 0 < port <= %d", math.MaxUint16)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)

	if flagREPL {
		log.Print("Entering REPL")
		go func() {
			errChan <- repl()
		}()
	} else {
		log.Print("Entering benchmark")
		go func() {
			errChan <- benchmark()
		}()
	}

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
