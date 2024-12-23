package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"go-prequel/client"
	"go-prequel/server"
	"fmt"
	"math/rand"
)

func main() {
	mode := flag.String("mode", "", "Mode to run: server or client")
	port := flag.String("port", "8080", "Port to run the server on (server mode only)")
	configPath := flag.String("config", "", "Path to the config file (client mode only)")
	flag.Parse()

	switch *mode {
	case "server":
		runServer(*port)
	case "client":
		runClient(*configPath)
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

func runClient(configPath string) {
	file, err := os.Open(configPath)
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	defer file.Close()

	var config client.Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		log.Fatalf("Failed to decode config file: %v", err)
	}

	c := client.NewClient(config, config.Servers)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Channel to listen for OS signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			for i := 0; i < 4; i++ {
				go func() {
					switch rand.Intn(3) {
					case 0:
						if err := c.Ping(); err != nil {
							log.Printf("Ping failed: %v", err)
						} else {
							log.Println("Ping successful")
						}
					case 1:
						if err := c.BatchProcess([]string{"example"}); err != nil {
							log.Printf("BatchProcess failed: %v", err)
						} else {
							log.Println("BatchProcess successful")
						}
					case 2:
						if err := c.MediumProcess(); err != nil {
							log.Printf("MediumProcess failed: %v", err)
						} else {
							log.Println("MediumProcess successful")
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
