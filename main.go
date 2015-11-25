package main

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/pengux/pub-sub/pubsub"
)

func main() {
	ps := pubsub.New()

	router := httprouter.New()
	router = ps.SetupRoutes(router)

	log.Fatal(http.ListenAndServe(":8080", router))
}
