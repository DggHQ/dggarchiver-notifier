package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	model "github.com/DggHQ/dggarchiver-model"
	"github.com/nats-io/nats.go"
)

var (
	streaming bool
)

/*
The main function. Here we run the API checks in the background and will notify the message bus about the livestream status
*/
func main() {
	// We use waitgroups to run infinitely
	wg := sync.WaitGroup{}

	nc, err := nats.Connect("10.10.90.70", nil, nats.PingInterval(20*time.Second), nats.MaxPingsOutstanding(5))
	if err != nil {
		log.Fatalln(err)
	}

	// Run the api check forever
	go checkAPI(nc)
	// Add one routine to the waitgroup
	wg.Add(1)
	// Wait forever since we wont be sending wg.Done() to the waitgroup
	wg.Wait()
}

/*
We use the api client to periodically check the dgg api for the livestream status.
*/
func checkAPI(nc *nats.Conn) {
	for {
		ctx := context.Background()
		c := NewClient()
		info, err := c.GetStreamInfo(ctx)
		if err != nil {
			log.Fatalln(err)
		}
		log.Printf("Stream live: %v", info.Data.Streams.Youtube.Live)
		streaming = info.Data.Streams.Youtube.Live

		// Publish request to the worker queue and expect a reply from a worker.
		// Once a worker that is subscribed to the queue accepts the request it sends a message to the reply topic.
		// The notifier will not send any more data to the stream.live topic as long as the download is running.
		// Once the download is complete, send another request to the stream.live topic that unmutes the notifier.
		// This would require some kind of queue management.

		data, err := json.Marshal(
			model.LiveNotify{
				Live: streaming,
			})
		if err != nil {
			log.Fatalln(err)
		}

		req := nats.Msg{
			Subject: "stream.live",
			Data:    data,
		}
		nc.PublishMsg(&req)

		// nc.QueueSubscribe("stream.live", "worker", func(msg *nats.Msg) {
		// 	msg.Respond()
		// })

		time.Sleep(time.Minute)
	}
}
