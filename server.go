package lruchal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/net/netutil"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Item struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
	TTL   string      `json:"ttl"`
}

const (
	DefaultPort            = 8182
	DefaultCacheSize       = 1000
	DefaultConnectionLimit = 50
)

type ServerConfig struct {
	Port            int // port to present http api to
	ConnectionLimit int // maximum number of concurrent connections to perform
	CacheSize       int // maximum number of records allowable in cache
	Logger          Logger
}

func NewDefaultServerConfig() *ServerConfig {
	c := &ServerConfig{
		Port:            DefaultPort,
		CacheSize:       DefaultCacheSize,
		ConnectionLimit: DefaultConnectionLimit,
		Logger:          DefaultLogger("server"),
	}

	return c
}

type Server struct {
	mu       *sync.Mutex
	ctx      context.Context
	log      Logger
	cache    Cache
	listener net.Listener
	running  bool
}

func NewDefaultServer() (*Server, error) {
	return newServer(NewDefaultServerConfig())
}

func NewServer(config *ServerConfig) (*Server, error) {
	return newServer(config)
}

func newServer(config *ServerConfig) (*Server, error) {
	var err error

	def := NewDefaultServerConfig()
	if config.Port > 0 {
		def.Port = config.Port
	}
	if config.CacheSize > 0 {
		def.CacheSize = config.CacheSize
	}
	if config.ConnectionLimit > 0 {
		def.ConnectionLimit = config.ConnectionLimit
	}
	if config.Logger != nil {
		def.Logger = config.Logger
	}

	srv := &Server{
		mu:    new(sync.Mutex),
		ctx:   context.Background(),
		log:   def.Logger,
		cache: NewMemoryCache(def.CacheSize),
	}

	tcp, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", def.Port))
	if err != nil {
		return nil, fmt.Errorf("unable to resolve tcp addr: %s", err)
	}

	listener, err := net.ListenTCP("tcp", tcp)
	if err != nil {
		return nil, fmt.Errorf("unable to listen: %s", err)
	}

	srv.listener = netutil.LimitListener(listener, def.ConnectionLimit)

	return srv, nil
}

func (srv *Server) Serve() error {
	srv.mu.Lock()
	if srv.running {
		return errors.New("server already running")
	}
	srv.running = true
	srv.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handle)

	return http.Serve(srv.listener, mux)
}

func (srv *Server) handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		srv.get(w, r)
	case "PUT":
		srv.put(w, r)
	default:
		defer r.Body.Close()
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
}

func (srv *Server) get(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	split := strings.Split(r.RequestURI, "/")
	if len(split) != 3 {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if split[1] != "get" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if value := srv.cache.Get(split[2]); value == nil {
		http.Error(w, fmt.Sprintf("Key \"%s\" not found", split[2]), http.StatusNotFound)
	} else if b, err := json.Marshal(value); err != nil {
		http.Error(w, fmt.Sprintf("Unable to marshal value: %s", err), http.StatusUnprocessableEntity)
	} else {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(b)))
		w.Write(b)
	}
}

func (srv *Server) put(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if r.RequestURI != "/put" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to read body: %s", err), http.StatusUnprocessableEntity)
		return
	}

	item := new(Item)
	err = json.Unmarshal(b, item)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to unmarshal body: %s", err), http.StatusUnprocessableEntity)
		return
	}

	duration, err := time.ParseDuration(item.TTL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid TTL format specified: %s", err), http.StatusNotAcceptable)
		return
	}

	srv.cache.Put(item.Key, item.Value, duration)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusNoContent)
}
