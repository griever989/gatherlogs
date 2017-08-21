package main

import (
	log "github.com/sirupsen/logrus"
	"net/rpc"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/griever989/gatherlogs/common"
	"github.com/mattn/go-colorable"
)

func connect(target string) *rpc.Client {
	client, err := rpc.DialHTTP("tcp", target)
	if err != nil {
		log.Fatal("failed to connect to ", target)
	}
	log.Print("connected to ", target)
	return client
}

func configLogger() {
	log.SetFormatter(&log.TextFormatter{ForceColors: true})
	log.SetOutput(colorable.NewColorableStdout())
	log.SetLevel(log.DebugLevel)
}

func testSend(client *rpc.Client) {
	var response bool
	msg := common.LogMessage{Message: "test", Time: &timestamp.Timestamp{}}
	err := client.Call("Gatherer.Send", &msg, &response)
	if err != nil {
		log.Fatal("failed to send ", err)
	}
	log.Debug("sent", msg)
}

func testSendMulti(client *rpc.Client) {
	var response bool
	msgs := make([]common.LogMessage, 20)
	msg := common.LogMessage{Message: "test", Time: &timestamp.Timestamp{}}
	for i := range msgs {
		msgs[i] = msg
	}
	err := client.Call("Gatherer.SendMultiple", &msgs, &response)
	if err != nil {
		log.Fatal("failed to send ", err)
	}
	log.Debug("sent multiple", msg)
}

func main() {
	configLogger()
	client := connect("localhost:9494")
	testSend(client)
	testSendMulti(client)
}
