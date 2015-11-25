package pubsub

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
)

type (
	PubSub struct {
		// topics is a nested map containing subscribers and slices
		// of Message for each topic
		topics map[string]subscriptions
		sync.Mutex
	}

	Message struct {
		Content   string    `json:"message"`
		Published time.Time `json:"published,omitempty"`
	}

	subscriptions map[string][]Message
)

// New returns a new PubSub instance with a Nats encoded connection
func New() *PubSub {
	return &PubSub{topics: make(map[string]subscriptions)}
}

// SetupRoutes maps routes to the PubSub's handlers
func (ps *PubSub) SetupRoutes(router *httprouter.Router) *httprouter.Router {
	router.POST("/:topic_name", ps.PublishMessage)
	router.POST("/:topic_name/:subscriber_name", ps.Subscribe)
	router.DELETE("/:topic_name/:subscriber_name", ps.Unsubscribe)
	router.GET("/:topic_name/:subscriber_name", ps.GetMessages)

	return router
}

// PublishMessage send a message to all subscribers
// POST /:topic_name
func (ps *PubSub) PublishMessage(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	topic := p.ByName("topic_name")

	ps.Lock()
	defer ps.Unlock()

	// If there is no subscribers to the topic, just return empty response
	if _, ok := ps.topics[topic]; !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var msg Message
	err := unmarshalBody(r, &msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set message published time to server's time
	msg.Published = time.Now()

	for subscriber, _ := range ps.topics[topic] {
		ps.topics[topic][subscriber] = append(ps.topics[topic][subscriber], msg)
	}

	w.WriteHeader(http.StatusCreated)
}

// Subscribe adds a subscription to a topic. The subscriber will not receive
// previously published messages for the topic.
// POST /:topic_name/:subscriber_name
func (ps *PubSub) Subscribe(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	topic, subscriber := p.ByName("topic_name"), p.ByName("subscriber_name")

	ps.Lock()
	defer ps.Unlock()
	if _, ok := ps.topics[topic]; !ok {
		ps.topics[topic] = make(map[string][]Message)
	}

	if _, ok := ps.topics[topic][subscriber]; !ok {
		ps.topics[topic][subscriber] = make([]Message, 0)
	}

	w.WriteHeader(http.StatusCreated)
}

// Unsubscribe removes a subscription of a topic.
// DELETE /:topic_name/:subscriber_name
func (ps *PubSub) Unsubscribe(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	topic, subscriber := p.ByName("topic_name"), p.ByName("subscriber_name")

	ps.Lock()
	defer ps.Unlock()
	if _, ok := ps.topics[topic]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if _, ok := ps.topics[topic][subscriber]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	delete(ps.topics[topic], subscriber)

	// If there are no more subscribers left, remove the topic too
	if len(ps.topics[topic]) == 0 {
		delete(ps.topics, topic)
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetMessages returns published messages to the subscriber since subscription.
// GET /:topic_name/:subscriber_name
func (ps *PubSub) GetMessages(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	topic, subscriber := p.ByName("topic_name"), p.ByName("subscriber_name")

	ps.Lock()
	defer ps.Unlock()
	if _, ok := ps.topics[topic]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if _, ok := ps.topics[topic][subscriber]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if len(ps.topics[topic][subscriber]) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	body, err := json.Marshal(ps.topics[topic][subscriber])
	if err != nil {
		http.Error(w, "Could not marshal messages in response", http.StatusInternalServerError)
		return
	}

	_, err = w.Write(body)
	if err != nil {
		http.Error(w, "Could not write body of response", http.StatusInternalServerError)
		return
	}

	// empty the message queue for the subscription
	ps.topics[topic][subscriber] = make([]Message, 0)

	w.WriteHeader(http.StatusOK)
}

// unmarshalBody unmarshal JSON data in body of requests to structs
func unmarshalBody(r *http.Request, object interface{}) error {
	if r.Body == nil {
		return nil
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, object)
	if err != nil {
		return err
	}

	return nil
}
