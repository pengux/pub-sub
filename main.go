package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/nats-io/gnatsd/server"
	"github.com/nats-io/nats"
	"github.com/pengux/pub-sub/pubsub"
)

var GnatsdOptions = server.Options{
	Host:   "localhost",
	Port:   4222,
	NoLog:  true,
	NoSigs: true,
}

func main() {
	c, err := NewGnatsdConn(&GnatsdOptions)
	if err != nil {
		log.Fatal("Could not create an encoded connection to Gnatds server")
	}
	defer c.Close()

	ps := pubsub.New(c)

	router := httprouter.New()
	router = ps.SetupRoutes(router)

	log.Fatal(http.ListenAndServe(":8080", router))
}

func NewGnatsdConn(opts *server.Options) (*nats.EncodedConn, error) {
	RunGnatsd(opts)

	nc, err := nats.Connect(fmt.Sprintf("nats://%s:%d", GnatsdOptions.Host, GnatsdOptions.Port))
	if err != nil {
		log.Fatal("Could not connect to Gnatsd server")
	}

	return nats.NewEncodedConn(nc, nats.JSON_ENCODER)
}

// RunGnatsd will start a Gnatsd server with the passed options
func RunGnatsd(opts *server.Options) *server.Server {
	s := server.New(opts)
	if s == nil {
		log.Fatal("No NATS Server object returned.")
	}

	// Run server in Go routine.
	go s.Start()

	end := time.Now().Add(10 * time.Second)
	for time.Now().Before(end) {
		addr := s.Addr()
		if addr == nil {
			time.Sleep(50 * time.Millisecond)
			// Retry. We might take a little while to open a connection.
			continue
		}
		conn, err := net.Dial("tcp", addr.String())
		if err != nil {
			// Retry after 50ms
			time.Sleep(50 * time.Millisecond)
			continue
		}
		conn.Close()
		return s
	}

	log.Fatal("Unable to start NATS Server in Go Routine")

	return nil
}
