package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Location struct {
	Latitude  interface{} `json:"lat"`
	Longitude interface{} `json:"lon"`
}

type Backend struct {
	URL          *url.URL
	IP           string
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

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	// Earth radius in kilometers
	const earthRadius = 6371

	// Convert latitude and longitude from degrees to radians
	lat1Rad := lat1 * (math.Pi / 180)
	lon1Rad := lon1 * (math.Pi / 180)
	lat2Rad := lat2 * (math.Pi / 180)
	lon2Rad := lon2 * (math.Pi / 180)

	// Calculate differences
	deltaLat := lat2Rad - lat1Rad
	deltaLon := lon2Rad - lon1Rad

	// Haversine formula
	a := math.Pow(math.Sin(deltaLat/2), 2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Pow(math.Sin(deltaLon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	// Distance calculation
	distance := earthRadius * c
	return distance
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
	fmt.Println("Load balancing across the following backends:")
	//for _, server := range servers {
	//	parsedURL, err := url.Parse(server)
	//	if err != nil {
	//		log.Fatalf("Failed to parse URL %s: %v", server, err)
	//	}
	//	ipOfServers := parsedURL.Hostname()
	//	coordinatesOfServer, err := getCoordinatesFromIP(ipOfServers)
	//	fmt.Println(coordinatesOfServer)
	//}

	serverPool := &ServerPool{}

	for _, server := range servers {
		serverURL, err := url.Parse(server)
		if err != nil {
			log.Fatal(err)
		}

		ip := serverURL.Hostname()
		proxy := httputil.NewSingleHostReverseProxy(serverURL)
		alive := healthCheck(server)
		backend := &Backend{
			URL:          serverURL,
			IP:           ip,
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

	ipOfLB := getPublicIP()
	fmt.Println("\x1b[31mINFORMATION OF LOAD BALANCER\x1b[0m")
	fmt.Println("Your public IP address is:", ipOfLB)
	coordinatesOfLB, err := getCoordinatesFromIP(ipOfLB)
	if err != nil {
		log.Fatal("Failed to get coordinates:", err)
	}

	fmt.Printf("Coordinates for IP %s: Latitude %f, Longitude %f\n", ipOfLB, coordinatesOfLB.Latitude, coordinatesOfLB.Longitude)

	lbLat := coordinatesOfLB.Latitude.(float64)
	lbLon := coordinatesOfLB.Longitude.(float64)

	for _, server := range servers {
		fmt.Println("\x1b[31mINFORMATION OF SERVER\x1b[0m")
		parsedURL, err := url.Parse(server)
		if err != nil {
			log.Fatalf("Failed to parse URL %s: %v", server, err)
		}
		ip := parsedURL.Hostname()
		coordinatesOfServer, err := getCoordinatesFromIP(ip)
		fmt.Printf("Location for Server %s: Latitude %f, Longitude %f\n", ip, coordinatesOfServer.Latitude, coordinatesOfServer.Longitude)
		if err != nil {
			log.Fatalf("Failed to get coordinates for server %s: %v", ip, err)
		}

		serverLat := coordinatesOfServer.Latitude.(float64)
		serverLon := coordinatesOfServer.Longitude.(float64)

		// Calculate distance between load balancer and server
		distance := haversine(lbLat, lbLon, serverLat, serverLon)
		fmt.Printf("Distance between load balancer and server %s: %.2f km\n", ip, distance)

		if distance <= 1 {
			fmt.Printf("\x1b[32mServer status: Good\x1b[0m\n\n")
		} else if distance <= 2 {
			fmt.Printf("\x1b[33mServer status: advantage\x1b[0m\n\n")
		} else {
			fmt.Printf("\x1b[31mServer status: Bad\x1b[0m\n\n")
		}
	}

	log.Printf("Load Balancer started at :%d\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func getPublicIP() string {
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		log.Fatal("Failed to get public IP:", err)
	}
	defer resp.Body.Close()

	ip, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed to read response:", err)
	}

	return string(ip)
}

func getCoordinatesFromIP(ip string) (Location, error) {
	url := fmt.Sprintf("https://nominatim.openstreetmap.org/search?format=json&q=%s", ip)
	resp, err := http.Get(url)
	if err != nil {
		return Location{}, err
	}
	defer resp.Body.Close()

	var locations []Location
	err = json.NewDecoder(resp.Body).Decode(&locations)
	if err != nil {
		return Location{}, err
	}

	if len(locations) == 0 {
		return Location{}, fmt.Errorf("no coordinates found for IP %s", ip)
	}

	latitude := 0.0
	switch v := locations[0].Latitude.(type) {
	case float64:
		latitude = v
	case string:
		lat, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Location{}, err
		}
		latitude = lat
	}

	longitude := 0.0
	switch v := locations[0].Longitude.(type) {
	case float64:
		longitude = v
	case string:
		lon, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Location{}, err
		}
		longitude = lon
	}

	return Location{
		Latitude:  latitude,
		Longitude: longitude,
	}, nil
}
