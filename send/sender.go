package main

import (
	"log"
	"net/rpc"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/griever989/gatherlogs/common"
)

func connect(target string) *rpc.Client {
	client, err := rpc.DialHTTP("tcp", target)
	if err != nil {
		log.Fatal("failed to connect to " + target)
	}
	log.Print("connected")
	return client
}

func main() {
	client := connect("localhost:9494")
	var response bool
	msg := common.LogMessage{Message: "test", Time: &timestamp.Timestamp{}}
	err := client.Call("Gatherer.Send", &msg, &response)
	if err != nil {
		log.Fatal("failed to send")
	}
	log.Print("sent", msg)
}
