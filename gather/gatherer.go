package main

import (
	// "errors"
	"log"
	"net"
	"net/http"
	"net/rpc"

	"github.com/griever989/gatherlogs/common"
)

var protocol = "tcp"
var port = ":9494"

// Gatherer receives log messages
type Gatherer struct {
	receiveQueue chan common.LogMessage
}

// NewGatherer initializes a Gatherer
func NewGatherer() *Gatherer {
	return &Gatherer{receiveQueue: make(chan common.LogMessage)}
}

// Send sends a log message
func (g *Gatherer) Send(msg common.LogMessage, reply *bool) error {
	log.Print("send called with", msg)
	g.receiveQueue <- msg
	log.Print("written to queue")
	return nil
}

func serve() *Gatherer {
	g := NewGatherer()
	rpc.Register(g)
	rpc.HandleHTTP()
	l, e := net.Listen(protocol, port)
	if e != nil {
		log.Fatal("failed to bind " + port)
	}
	log.Print("bound " + port)
	go http.Serve(l, nil)
	log.Print("serving as http")
	return g
}

func consume(g *Gatherer) {
	for {
		log.Print("consumed queue message ", <-g.receiveQueue)
	}
}

func main() {
	g := serve()
	consume(g)
}
