package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"net"
	"runtime"
	"github.com/shirou/gopsutil/cpu"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, World!")
	// run some CPU-bound work
	// for i := 0; i < 100000000; i++ {
	// 	_ = i
	// }
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)

    numCPU := runtime.NumCPU()

    fmt.Fprintf(w, "CPU: %d\n", numCPU)
    fmt.Fprintf(w, "Allocated Memory: %d bytes\n", memStats.Alloc)
    fmt.Fprintf(w, "Total Memory: %d bytes\n", memStats.Sys)
}

func cpuHandler(w http.ResponseWriter, r *http.Request) {
    cpuPercent, _ := cpu.Percent(0, false)
    fmt.Fprintf(w, "CPU usage: %.2f%%\n", cpuPercent[0])
}

func main() {
	http.HandleFunc("/", helloHandler)
	http.HandleFunc("/stats", statsHandler) // new handler for server stats
    http.HandleFunc("/cpu", cpuHandler) // new handler for CPU usage

	listener, err := net.Listen("tcp", "localhost:0") 
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to listen: %v\n", err)
		os.Exit(1)
	}

	serverURL := url.URL{
		Scheme: "http",
		Host:   listener.Addr().String(),
	}

	go func() {
		_, err = http.Get("http://localhost:8080/register?url=" + url.QueryEscape(serverURL.String()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to register with load balancer: %v\n", err)
			os.Exit(1)
		}
	}()

	fmt.Printf("Server is listening on %s\n", listener.Addr().String())
	http.Serve(listener, nil)
}