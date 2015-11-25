package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/pengux/pub-sub/pubsub"
)

var router *httprouter.Router

func TestPubSub(t *testing.T) {
	ps := pubsub.New()

	router = httprouter.New()
	router = ps.SetupRoutes(router)

	topic := "pubsub"
	subscriber1 := "me"
	subscriber2 := "me2"
	message := "variable content string"
	var msgs []pubsub.Message

	// Publish without subscribers, this message should not be delivered after subscription
	rec, _ := newRequest("POST", fmt.Sprintf("/%s", topic), strings.NewReader(`{"message": "should not be delivered"}`))
	if rec.Code != http.StatusNoContent {
		t.Errorf("publishing to topic, expecting status code %d, got %d and body %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	// Subscribe with subscriber1
	rec, _ = newRequest("POST", fmt.Sprintf("/%s/%s", topic, subscriber1), nil)
	if rec.Code != http.StatusCreated {
		t.Errorf("subscribing to topic with subscriber, expecting status code %d, got %d and body %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	// Subscribe with subscriber2
	rec, _ = newRequest("POST", fmt.Sprintf("/%s/%s", topic, subscriber2), nil)
	if rec.Code != http.StatusCreated {
		t.Errorf("subscribing to topic with subscriber2, expecting status code %d, got %d and body %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	// Publish
	rec, _ = newRequest("POST", fmt.Sprintf("/%s", topic), strings.NewReader(`{"message": "`+message+`"}`))
	if rec.Code != http.StatusCreated {
		t.Errorf("publishing to topic, expecting status code %d, got %d and body %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	// Polling with subscriber
	rec, _ = newRequest("GET", fmt.Sprintf("/%s/%s", topic, subscriber1), nil)
	if rec.Code != http.StatusOK {
		t.Errorf("polling messages from topic with subscriber1, expecting status code %d, got %d and body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	err := json.Unmarshal(rec.Body.Bytes(), &msgs)
	if err != nil {
		t.Errorf("unmarshal message from topic with subscriber1, got %s", err.Error())
	}
	if len(msgs) != 1 {
		t.Errorf("polling messages from topic with subscriber1, expecting message count to be 1 got %s", len(msgs))
	}
	if msgs[0].Content != message {
		t.Errorf("polling messages from topic with subscriber1, expecting message to be %s, got %s", message, msgs[0].Content)
	}
	if msgs[0].Published.IsZero() {
		t.Errorf("polling messages from topic with subscriber1, expecting published to NOT be zero value")
	}

	// Polling with subscriber2
	rec, _ = newRequest("GET", fmt.Sprintf("/%s/%s", topic, subscriber2), nil)
	if rec.Code != http.StatusOK {
		t.Errorf("polling messages from topic with subscriber2, expecting status code %d, got %d and body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	err = json.Unmarshal(rec.Body.Bytes(), &msgs)
	if err != nil {
		t.Errorf("unmarshal message from topic with subscriber2, got %s", err.Error())
	}
	if len(msgs) != 1 {
		t.Errorf("polling messages from topic with subscriber2, expecting message count to be 1 got %s", len(msgs))
	}
	if msgs[0].Content != message {
		t.Errorf("polling messages from topic with subscriber2, expecting message to be %s, got %s", message, msgs[0].Content)
	}
	if msgs[0].Published.IsZero() {
		t.Errorf("polling messages from topic with subscriber2, expecting published to NOT be zero value")
	}

	// Polling again should return 204 and an empty array
	rec, _ = newRequest("GET", fmt.Sprintf("/%s/%s", topic, subscriber1), nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("polling messages from topic, expecting status code %d, got %d and body %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "" {
		t.Errorf("polling messages from topic again, expecting empty body, got %s", rec.Body.String())
	}

	// Unsubscribe
	rec, _ = newRequest("DELETE", fmt.Sprintf("/%s/%s", topic, subscriber1), nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("unsubscribing from topic, expecting status code %d, got %d and body %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	// Polling after unsubscribe should return 404
	rec, _ = newRequest("GET", fmt.Sprintf("/%s/%s", topic, subscriber1), nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("polling messages from topic, expecting status code %d, got %d and body %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}
}

func newRequest(method, url string, body io.Reader) (*httptest.ResponseRecorder, *http.Request) {
	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(method, url, body)

	router.ServeHTTP(rec, req)

	return rec, req
}
