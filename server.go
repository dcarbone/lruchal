package lruchal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultPort            = 8182
	DefaultCacheSize       = 1000
	DefaultConnectionLimit = 50
)

type serverActionType uint8

const (
	serverActionGet serverActionType = iota
	serverActionPut
)

type serverAction interface {
	Type() serverActionType
	Act(*Cache)
}

type actionGet struct {
	key string
	w   http.ResponseWriter
	r   *http.Request
}

func (g *actionGet) Type() serverActionType {
	return serverActionGet
}

func (g *actionGet) Act(cache *Cache) {
	if value := cache.Get(g.key); value == nil {
		http.Error(g.w, fmt.Sprintf("Key \"%s\" not found", g.key), http.StatusNotFound)
	} else if b, err := json.Marshal(value); err != nil {
		http.Error(g.w, fmt.Sprintf("Unable to marshal value: %s", err), http.StatusUnprocessableEntity)
	} else {
		g.w.Header().Set("Content-Type", "application/json; charset=utf-8")
		g.w.Header().Set("Content-Length", strconv.Itoa(len(b)))
		g.w.WriteHeader(http.StatusOK)
		g.w.Write(b)
	}
}

type actionPut struct {
	w http.ResponseWriter
	r *http.Request
}

func (p *actionPut) Type() serverActionType {
	return serverActionPut
}

func (p *actionPut) Act(cache *Cache) {
	b, err := ioutil.ReadAll(p.r.Body)
	if err != nil {
		http.Error(p.w, fmt.Sprintf("Unable to read body: %s", err), http.StatusUnprocessableEntity)
		return
	}

	item := new(Item)
	err = json.Unmarshal(b, item)
	if err != nil {
		http.Error(p.w, fmt.Sprintf("Unable to unmarshal body: %s", err), http.StatusUnprocessableEntity)
		return
	}

	duration, err := time.ParseDuration(item.TTL)
	if err != nil {
		http.Error(p.w, fmt.Sprintf("Invalid TTL format specified: %s", err), http.StatusNotAcceptable)
		return
	}

	cache.Put(item.Key, item.Value, duration)
	p.w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	p.w.Header().Set("Content-Length", "0")
	p.w.WriteHeader(http.StatusNoContent)
}

type ServerConfig struct {
	Port            int // port to present http api to
	ConnectionLimit int // maximum number of concurrent connections to handle
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
	cache    *Cache
	listener *net.TCPListener
	running  bool
	acts     chan serverAction
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
		cache: NewCache(def.CacheSize),
		acts:  make(chan serverAction, def.ConnectionLimit),
	}

	tcp, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", def.Port))
	if err != nil {
		return nil, fmt.Errorf("unable to resolve tcp addr: %s", err)
	}

	srv.listener, err = net.ListenTCP("tcp", tcp)
	if err != nil {
		return nil, fmt.Errorf("unable to listen: %s", err)
	}

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
	mux.HandleFunc("/get", srv.get)
	mux.HandleFunc("/put", srv.put)

	return http.Serve(srv.listener, mux)
}

func (srv *Server) get(w http.ResponseWriter, r *http.Request) {
	split := strings.Split(r.RequestURI, "/")
	if len(split) != 2 {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	select {
	case srv.acts <- &actionGet{split[1], w, r}:
	default:
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
}

func (srv *Server) put(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI != "/put" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if r.Method != "PUT" {
		http.Error(w, http.StatusText(http.StatusNotAcceptable), http.StatusNotAcceptable)
		return
	}

	select {
	case srv.acts <- &actionPut{w, r}:
	default:
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
	}
}

func (srv *Server) handle() {
	for act := range srv.acts {
		act.Act(srv.cache)
	}
}
