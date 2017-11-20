# lruchal

This package is not particularly useful.

## Architecture

1. I chose to not include any 3rd party packages as I've not written an LRU before and wanted to have some fun
1. I chose to use an in-memory cache as without something to actually implement, I did not feel any specific backend
had an advantage.

## Maintainability

This package will be easy to maintain as it's pretty simple, the one exception being relying on the experimental
package `golang.org/x/net` because implementing the semaphore is boring.

## Tests

Executing `make test` will spin up two containers, one for tests and one for bench.  The `-race` flag cannot be tested 
within alpine at the moment ([see here](https://github.com/golang/go/issues/14481)), and I don't feel it's worth 
spinning up an entire Ubuntu/etc. container just for one test.  Here is the output of `-race` just for funsies:

```
go test -race
PASS
ok      github.com/dcarbone/lruchal     1.008s
```

## Complexity

This lib has pert-near `O(1)` complexity, as evidenced by this output:

```
go test -run=_nothing_ -bench=.
goos: linux
goarch: amd64
pkg: github.com/dcarbone/lruchal
BenchmarkMemoryCache100-8       2000000000               0.03 ns/op
BenchmarkMemoryCache200-8       2000000000               0.06 ns/op
BenchmarkMemoryCache1000-8      2000000000               0.33 ns/op
PASS
ok      github.com/dcarbone/lruchal     20.901s
```

This is fairly easy to understand, as each interaction with the cache acquires a lock and therefore only 1
routine can directly interact with the cache at a time.

## Scalability

This varies greatly, it is designed currently to run as a single node and it's suitability will depend entirely upon
the application.  If more than 1 node is required and each node must be aware of the same data, I could add serf or
something to sync data across a quorum.

## Instructions

From the root dir, execute `docker-compose up --build`.  This will spin up 2 containers, one server, one client.  The
client will seed the server with 100 keys with a ttl of 5m.  You may then either use postman or whatever http client
you like to interact with the server.

### API Interaction

#### /get/{key}

Does just that.  curl:
`curl "http://127.0.0.1:8182/get/key1"`

#### /put (HTTP PUT)

Will store a value in the cache.  An example curl command: 
`curl -X PUT -d '{"key": "key1", "value": "value1", "ttl": "10m"}' "http://127.0.0.1:8182/put"`

### Client REPL

Optionally, if you'd like and have go installed, [client](./client.go) has a repl mode.

1. go get golang.org/x/net
1. cd into [./client](./client)
1. go build
1. ./client -repl

There are 2 allowable commands:

1. `get {keyname}`
1. `put -k {key} -v {value} -ttl {ttl}`

## Optimizations

I mean the entire point of this is the caching layer, so I would try to find one that is suitable for the application
it would be used for.  Other than that, the rest of this package is itty bitty.  If it were in actual production use 
some sort of auth system should be put in place.  Probably try to find a less aggressive locking mechanism.

## Final Thoughts

I had a lot of fun building this.  Hopefully it isn't terrible.