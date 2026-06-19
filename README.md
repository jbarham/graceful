# graceful

`graceful` is a small Go library that provides a drop-in replacement for the standard
[http.ListenAndServe](https://pkg.go.dev/net/http#ListenAndServe) function to run HTTP servers with graceful shutdown capabilities.

## Installation

`go get github.com/jbarham/graceful`

## Motivation and Usage

Why does graceful shutdown matter? Because by default, Go HTTP servers will terminate in-flight requests when they're stopped
by signals like SIGTERM or SIGINT. This can have unwanted consequences in production systems if those terminated requests were
midway through operations such as updating a customer account.

The following sample code illustrates exactly how `graceful` works to gracefully shut down Go HTTP servers without terminating
in-flight requests:

```go
package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jbarham/graceful"
)

var runGracefully = flag.Bool("graceful", false, "set to run gracefully")

func main() {
	flag.Parse()

	http.HandleFunc("GET /{delay}", func(w http.ResponseWriter, r *http.Request) {
		delay, err := strconv.Atoi(r.PathValue("delay"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("Starting request with %ds delay...", delay)
		time.Sleep(time.Duration(delay) * time.Second)
		log.Print("Finished request")
	})

	if *runGracefully {
		graceful.ListenAndServe(":8080", nil)
	} else {
		http.ListenAndServe(":8080", nil)
	}
}
```

Save it to a file named, say `main.go`, and run `go run main.go` to start the HTTP server without graceful shutdown.
Load http://localhost:8080/5 to trigger a 5 second request, then stop the server with **Ctrl-C** and note that the in-flight
request is terminated.

Run it again with `go run main.go -graceful`, repeat the above steps and note that the in-flight request is allowed
to finish before the server shuts down.

## Reference

https://pkg.go.dev/github.com/jbarham/graceful
