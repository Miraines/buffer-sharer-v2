package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	flag.Parse()

	// Validate port number
	if *port < 1 || *port > 65535 {
		log.Fatalf("Invalid port number: %d. Port must be between 1 and 65535", *port)
	}

	addr := fmt.Sprintf(":%d", *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()

	log.Printf("╔════════════════════════════════════════╗")
	log.Printf("║     Buffer Sharer Middleware v5.4      ║")
	log.Printf("╠════════════════════════════════════════╣")
	log.Printf("║  Listening on %-24s ║", addr)
	log.Printf("╠════════════════════════════════════════╣")
	log.Printf("║  Room-based relay server               ║")
	log.Printf("║  Controller creates room -> gets code  ║")
	log.Printf("║  Client enters code -> joins room      ║")
	log.Printf("╚════════════════════════════════════════╝")
	log.Printf("")

	middleware := NewMiddleware()

	// Фоновые задачи
	go middleware.cleanupStaleRooms()
	go middleware.printStats()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down gracefully...")
		listener.Close()
		os.Exit(0)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}

		go middleware.HandleConnection(conn)
	}
}
