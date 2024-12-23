package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go-prequel/client"
	"go-prequel/metrics"
	"go-prequel/server"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	mode := flag.String("mode", "", "Mode to run: server or client")
	port := flag.String("port", "8080", "Port to run the server on (server mode only)")
	configPath := flag.String("config", "", "Path to the config file (client mode only)")
	selMode := flag.String("selection", "hcl", "Server selection mode (hcl/round_robin)")
	metricsPort := flag.String("metrics-port", "8099", "Port to run the metrics server on")

	flag.Parse()

	switch *mode {
	case "server":
		runServer(*port)
	case "client":
		runClient(*configPath, *selMode, *metricsPort)
	default:
		log.Fatalf("Invalid mode: %s. Use 'server' or 'client'.", *mode)
	}
}

func runServer(port string) {
	s := server.NewServer()
	addr := fmt.Sprintf("localhost:%s", port)
	err := s.Start(addr)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func collectMetrics(metricsPort string) {
	metrics.InitClientMetrics()
	metrics.StartMetricsServer("localhost:" + metricsPort)
}

func runClient(configPath string, selMode string, metricsPort string) {
	file, err := os.Open(configPath)
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	defer file.Close()

	var config client.Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		log.Fatalf("Failed to decode config file: %v", err)
	}

	c := client.NewClient(config, config.Servers, client.SelectionMode(selMode))

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Channel to listen for OS signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	collectMetrics(metricsPort)

	for {
		select {
		case <-ticker.C:
			for i := 0; i < 100; i++ {
				go func() {
					switch rand.Intn(3) {
					case 0:
						if err := c.Ping(); err != nil {
							log.Printf("Ping failed: %v", err)
						}
					case 1:
						if err := c.BatchProcess([]string{"example"}); err != nil {
							log.Printf("BatchProcess failed: %v", err)
						}
					case 2:
						if err := c.MediumProcess(); err != nil {
							log.Printf("MediumProcess failed: %v", err)
						}
					}
				}()
			}
		case <-sigs:
			log.Println("Received shutdown signal, stopping client...")
			c.Stop()
			return
		}
	}
}
