package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	listenAddr := flag.String("listen", "127.0.0.1:12345", "address in format host:port to listen to")
	upstreamAddr := flag.String("upstream", "1.1.1.1:22", "address of the upstream dns server")
	maxInflight := flag.Int("max-inflight", 64, "maximum number of client queries handled concurrently")
	flag.Parse()

	logger := log.New(io.Writer(os.Stderr), "", 0)

	conn, err := net.ListenPacket("udp4", *listenAddr)
	if err != nil {
		logger.Println("Failed to start the server", err)
		return
	}

	logger.Println("Server is running on", *listenAddr)

	server := newServer(conn, *upstreamAddr, logger, *maxInflight)
	go server.serve()

	defer conn.Close()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	<-signals

	logger.Printf("Exiting")
}
