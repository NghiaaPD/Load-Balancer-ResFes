package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Backend struct {
	URL          *url.URL
	Alive        bool
	ReverseProxy *httputil.ReverseProxy
	mu           sync.RWMutex
}

type ServerPool struct {
	backends []*Backend
}

func (s *ServerPool) AddBackend(backend *Backend) {
	s.backends = append(s.backends, backend)
}

func (s *ServerPool) HealthCheck() {
	for _, backend := range s.backends {
		alive := healthCheck(backend.URL.String())
		backend.mu.Lock()
		backend.Alive = alive == "Good" || alive == "Average" // Consider "Good" and "Average" as alive
		backend.mu.Unlock()
	}
}

func (s *ServerPool) GetNextBackend() *Backend {
	var healthyBackends []*Backend
	for _, backend := range s.backends {
		backend.mu.RLock()
		if backend.Alive {
			healthyBackends = append(healthyBackends, backend)
		}
		backend.mu.RUnlock()
	}

	if len(healthyBackends) == 0 {
		return nil
	}

	next := rand.Intn(len(healthyBackends))
	return healthyBackends[next]
}

func healthCheck(url string) string {
	client := http.Client{
		Timeout: 2 * time.Second,
	}

	start := time.Now()
	resp, err := client.Get(url)
	if err != nil {
		return "Bad"
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)
	if elapsed.Seconds() < 0.5 {
		return "Good"
	} else if elapsed.Seconds() < 1 {
		return "Average"
	} else {
		return "Overloaded"
	}
}

func main() {
	var serverList string
	var port int
	flag.StringVar(&serverList, "backends", "", "Load balanced backends, use commas to separate")
	flag.IntVar(&port, "port", 3000, "Port to serve")
	flag.Parse()

	if len(serverList) == 0 {
		log.Fatal("Please provide one or more backends to load balance")
	}

	servers := strings.Split(serverList, ",")
	serverPool := &ServerPool{}

	for _, server := range servers {
		serverURL, err := url.Parse(server)
		if err != nil {
			log.Fatal(err)
		}

		proxy := httputil.NewSingleHostReverseProxy(serverURL)
		alive := healthCheck(server)
		backend := &Backend{
			URL:          serverURL,
			Alive:        alive == "Good" || alive == "Average", // Consider "Good" and "Average" as alive
			ReverseProxy: proxy,
		}
		serverPool.AddBackend(backend)
	}

	go func() {
		for {
			serverPool.HealthCheck()
			time.Sleep(5 * time.Second)
		}
	}()

	server := http.Server{
		Addr: fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer := serverPool.GetNextBackend()

			if peer != nil {
				peer.ReverseProxy.ServeHTTP(w, r)
				return
			}

			http.Error(w, "Service not available", http.StatusServiceUnavailable)
		}),
	}

	log.Printf("Load Balancer started at :%d\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
