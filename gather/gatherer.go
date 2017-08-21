package main

import (
	// "errors"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/rpc"

	"github.com/griever989/gatherlogs/common"
	"github.com/mattn/go-colorable"
)

var protocol = "tcp"
var port = ":9494"
var channelBufferSize = 1

// Gatherer receives log messages
type Gatherer struct {
	receiveQueue chan common.LogMessage
}

// NewGatherer initializes a Gatherer
func NewGatherer() *Gatherer {
	return &Gatherer{receiveQueue: make(chan common.LogMessage, channelBufferSize)}
}

// Send sends a log message
func (g *Gatherer) Send(msg common.LogMessage, reply *bool) error {
	log.Debug("send called with", msg)
	g.receiveQueue <- msg
	log.Debug("message written to queue")
	return nil
}

// SendMultiple sends many log messages
func (g *Gatherer) SendMultiple(msgs []common.LogMessage, reply *bool) error {
	log.Debug("send called with", msgs)
	for _, msg := range msgs {
		g.receiveQueue <- msg
	}
	log.Debug(len(msgs), " messages written to queue")
	return nil
}

func serve() *Gatherer {
	g := NewGatherer()
	rpc.Register(g)
	rpc.HandleHTTP()
	l, e := net.Listen(protocol, port)
	if e != nil {
		log.Fatal("failed to bind ", port, e)
	}
	log.Info("bound ", port)
	go http.Serve(l, nil)
	log.Info("serving as http")
	return g
}

func consume(g *Gatherer) {
	for {
		log.Debug("consumed queue message ", <-g.receiveQueue)
	}
}

func configLogger() {
	log.SetFormatter(&log.TextFormatter{ForceColors: true})
	log.SetOutput(colorable.NewColorableStdout())
	log.SetLevel(log.DebugLevel)
}

func main() {
	configLogger()
	g := serve()
	consume(g)
}
