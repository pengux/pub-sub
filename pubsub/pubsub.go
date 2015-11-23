package pubsub

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/nats-io/nats"
)

type (
	PubSub struct {
		conn *nats.EncodedConn
	}

	Message struct {
		Content   string    `json:"message"`
		Published time.Time `json:"published,omitempty"`
	}

	subscription struct {
		sub  *nats.Subscription
		msgs []Message
		sync.Mutex
	}
)

var (
	subscriptions = make(map[string]*subscription)
	mutex         = &sync.RWMutex{}
)

// New returns a new PubSub instance with a Nats encoded connection
func New(conn *nats.EncodedConn) *PubSub {
	return &PubSub{conn}
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
	var msg Message
	err := unmarshalBody(r, &msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set message published time to server's time
	msg.Published = time.Now()

	err = ps.conn.Publish(p.ByName("topic_name"), &msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// Subscribe adds a subscription to a topic. The subscriber will not receive
// previously published messages for the topic.
// POST /:topic_name/:subscriber_name
func (ps *PubSub) Subscribe(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	topic, subscriber := p.ByName("topic_name"), p.ByName("subscriber_name")
	mapKey := topic + "." + subscriber

	sub, err := ps.conn.Subscribe(topic, func(m *Message) {
		subscriptions[mapKey].Lock()
		subscriptions[mapKey].msgs = append(subscriptions[mapKey].msgs, *m)
		subscriptions[mapKey].Unlock()
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()
	if _, ok := subscriptions[mapKey]; !ok {
		subscriptions[mapKey] = &subscription{
			sub:  sub,
			msgs: make([]Message, 0),
		}
	}

	w.WriteHeader(http.StatusCreated)
}

// Unsubscribe removes a subscription of a topic.
// DELETE /:topic_name/:subscriber_name
func (ps *PubSub) Unsubscribe(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	mapKey := p.ByName("topic_name") + "." + p.ByName("subscriber_name")

	mutex.Lock()
	defer mutex.Unlock()
	if _, ok := subscriptions[mapKey]; ok {
		err := subscriptions[mapKey].sub.Unsubscribe()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		delete(subscriptions, mapKey)
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetMessages returns published messages to the subscriber since subscription.
// GET /:topic_name/:subscriber_name
func (ps *PubSub) GetMessages(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	mapKey := p.ByName("topic_name") + "." + p.ByName("subscriber_name")

	mutex.Lock()
	defer mutex.Unlock()
	if _, ok := subscriptions[mapKey]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	subscriptions[mapKey].Lock()
	defer subscriptions[mapKey].Unlock()
	if len(subscriptions[mapKey].msgs) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	body, err := json.Marshal(subscriptions[mapKey].msgs)
	subscriptions[mapKey].msgs = make([]Message, 0)
	if err != nil {
		http.Error(w, "Could not marshal messages in response", http.StatusInternalServerError)
		return
	}

	_, err = w.Write(body)
	if err != nil {
		http.Error(w, "Could not write body of response", http.StatusInternalServerError)
		return
	}

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
