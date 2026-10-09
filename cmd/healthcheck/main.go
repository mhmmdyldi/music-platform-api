// Command healthcheck probes the API's liveness endpoint and exits 0 when it
// answers 200. It exists for container HEALTHCHECKs: the runtime image is
// distroless and has no shell or curl.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:8080/livez", "endpoint to probe")
	timeout := flag.Duration("timeout", 3*time.Second, "request timeout")
	flag.Parse()

	client := &http.Client{Timeout: *timeout}
	resp, err := client.Get(*url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		os.Exit(1)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: %s returned %d\n", *url, resp.StatusCode)
		os.Exit(1)
	}
}
