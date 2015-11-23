# pub-sub
Example of pub-sub using [Gnatsd](https://github.com/nats-io/gnatsd).

## Description
NATS is a high performant message queue which can be used as a messaging platform for distributed apps. It can also be embedded into a Go application making it easy to prototype and test (no need to run a separate server).

## How to run
```sh
go get
go run main.go
```
then point your requests at [http://localhost:8080](http://localhost:8080)

to run the tests:
```sh
go test -race
```

## API

### Publisher:
- publishing: POST /:topic_name with JSON body as a message (response 204)

### Subscriber:
- subscribing: POST /:topic_name/:subscriber_name (response 201)
- unsubscribing: DELETE /:topic_name/:subscriber_name (response 204)
- polling: GET /:topic_name/:subscriber_name (200 with JSON body as a message or 404 if no subscription found or 204 if no new messages are available)

### Message format:
```
{
	"message" : "variable content string",
	"published" : "date" // returned only for polling
}
```

## TODO
- Add unit tests
- Add tests for concurrency
- Use in a demo chat application
