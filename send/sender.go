package main

import (
	"errors"
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/rpc"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/protobuf/ptypes"
	"github.com/griever989/gatherlogs/common"
	"github.com/hpcloud/tail"
	"github.com/mattn/go-colorable"
)

// if we support multiple clients, can have multiple mutexes here
var sendLock = sync.Mutex{}

func connect(target string) *rpc.Client {
	client, err := rpc.DialHTTP("tcp", target)
	if err != nil {
		log.Panicln("failed to connect to ", target)
	}
	log.Print("connected to ", target)
	return client
}

func configLogger() {
	log.SetFormatter(&log.TextFormatter{ForceColors: true})
	log.SetOutput(colorable.NewColorableStdout())
	log.SetLevel(log.DebugLevel)
}

func sendMessage(client *rpc.Client, msg common.LogMessage, maxTries int) {
	for i := 0; i < maxTries; i++ {
		success := func() bool {
			defer func() {
				r := recover()
				if r != nil {
					log.Warnln(r)
				}
			}()
			var response bool
			var err error
			func() {
				sendLock.Lock()
				defer sendLock.Unlock()
				err = client.Call("Gatherer.Send", &msg, &response)
			}()
			if err != nil {
				log.Warnln("failed to send ", err)
				return false
			}
			log.Debug("sent", msg)
			return true
		}()
		if success {
			return
		}
		time.Sleep(1 * time.Second)
	}
	panic(errors.New(fmt.Sprintf("Failed to send after %v tries", maxTries)))
}

func startTail(file string, done chan bool, client *rpc.Client) {
	tailer, err := tail.TailFile(file, tail.Config{Poll: true, Follow: true})
	if err != nil {
		log.Panicln("failed to tail", file)
	}
	log.Infoln("tailing file", file)
	defer tailer.Cleanup()
	for {
		select {
		case line := <-tailer.Lines:
			msg := common.LogMessage{Message: line.Text}
			sendMessage(client, msg, 10)
		case <-done:
			return
		}
	}
}

func startWatch(dir, ext string, client *rpc.Client) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Panicln("failed to open watcher on", dir)
	}
	err = watcher.Add(dir)
	if err != nil {
		log.Panicln("failed to open watcher on", dir)
	}
	tailing := make(map[string](chan bool))

	watchAndTail := func(file string) {
		tailing[file] = make(chan bool)
		go startTail(file, tailing[file], client)
	}

	// find existing files and tail them
	filepath.Walk(dir, func(path string, f os.FileInfo, _ error) error {
		if !f.IsDir() {
			if strings.HasSuffix(path, ext) {
				watchAndTail(path)
			}
		}
		return nil
	})

	// watch for new files and tail them
	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Create == fsnotify.Create {
				// new file created in dir
				if strings.HasSuffix(event.Name, ext) {
					watchAndTail(event.Name)
				}
			}
			if event.Op&fsnotify.Remove == fsnotify.Remove {
				// file deleted from dir
				if closeTail, ok := tailing[event.Name]; ok {
					// stop tailing the file
					closeTail <- true
					delete(tailing, event.Name)
				}
			}
		case err = <-watcher.Errors:
			log.Warnln("error", err)
		}
	}
}

func main() {
	configLogger()

	watchDir := flag.String("watch", "", "Directory to watch for file changes. Will send each line to the gatherer")
	watchExt := flag.String("ext", ".txt", "Extension of files to watch")
	gathererURI := flag.String("gatherer", "localhost:9494", "URI of the gatherer to send messages")
	send := flag.Bool("send", false, "Send message immediately with values -message, -time, -loglevel, -server")
	sendMsg := flag.String("message", "", "Message to send with -send flag")
	sendTime := flag.String("time", time.Time.Local(time.Now()).String(), "Time to send with -send flag in format RFC3339 -- defaults to local time")
	sendLogLevel := flag.String("loglevel", "DEBUG", "LogLevel to send with -send flag")
	sendServer := flag.String("server", "", "Server to send with -send flag")
	sendTries := flag.Int("tries", 10, "Times to try sending before panicing")

	flag.Parse()

	client := connect(*gathererURI)
	defer client.Close()

	if *send {
		time, _ := time.Parse(time.RFC3339Nano, *sendTime)
		protoTime, _ := ptypes.TimestampProto(time)
		msg := common.LogMessage{
			Message:  *sendMsg,
			Time:     protoTime,
			LogLevel: common.LogMessage_LogLevel(common.LogMessage_LogLevel_value[*sendLogLevel]),
			Server:   *sendServer}
		sendMessage(client, msg, *sendTries)
	} else if *watchDir != "" {
		startWatch(*watchDir, *watchExt, client)
	} else {
		log.Infoln("Nothing to do; exiting")
	}
}
