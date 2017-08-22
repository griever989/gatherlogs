package main

import (
	"database/sql"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/rpc"

	log "github.com/sirupsen/logrus"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/golang/protobuf/ptypes"
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
	log.Debug("send called with ", msg)
	g.receiveQueue <- msg
	log.Debug("message written to queue")
	return nil
}

// SendMultiple sends many log messages
func (g *Gatherer) SendMultiple(msgs []common.LogMessage, reply *bool) error {
	log.Debug("send called with ", msgs)
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

func consumeCli(g *Gatherer) {
	log.Info("writing queue to command-line")
	for {
		log.Debug("consumed queue message ", <-g.receiveQueue)
	}
}

func consumeDatabase(g *Gatherer, driver, connectionString, dbSchema, dbTable string, dbCreate bool) {
	db, err := sql.Open(driver, connectionString)
	if err != nil {
		log.Fatalln("failed to open db", driver, connectionString, err)
	}
	if !dbExists(db, dbSchema, dbTable) {
		if !dbCreate {
			log.Fatalln("database", dbSchema, dbTable, "does not exist and -dbCreate is not set. Provide -dbSchema and -dbTable as well as -dbCreate if you want to create the table automatically")
		}
		createDbTable(db, dbSchema, dbTable)
	}
	log.Info("writing queue to database")
	for {
		msg := <-g.receiveQueue
		_, err := db.Exec(`
			insert into `+fmt.Sprintf("[%v].[%v]", dbSchema, dbTable)+
			` (Server, LogLevel, Time, Message)
			values ($1, $2, $3, $4);`,
			msg.GetServer(), fmt.Sprintf("%v", msg.GetLogLevel()), ptypes.TimestampString(msg.GetTime()), msg.GetMessage())
		if err != nil {
			log.Warnln("Failed to insert message", msg, err)
		}
	}
}

func dbExists(db *sql.DB, dbSchema, dbTable string) bool {
	rows, err := db.Query("select * from INFORMATION_SCHEMA.TABLES where TABLE_SCHEMA = $1 and TABLE_NAME = $2", dbSchema, dbTable)
	if err != nil {
		log.Fatalln("failed to check for existence of", dbSchema, dbTable, err)
	}
	if rows.Next() {
		return true
	}
	return false
}

func createDbTable(db *sql.DB, dbSchema, dbTable string) {
	fullTable := fmt.Sprintf("[%v].[%v]", dbSchema, dbTable)
	_, err := db.Exec(`
		create table ` + fullTable + `
		(
			Id bigint NOT NULL IDENTITY (1,1) PRIMARY KEY NONCLUSTERED,
			Server nvarchar(255) NOT NULL,
			LogLevel nvarchar(255) NOT NULL,
			Time datetime2 NOT NULL,
			Message nvarchar(max) NULL
		);`)
	if err != nil {
		log.Fatalln("failed to create table", fullTable, err)
	}
	log.Infoln("created table", dbSchema, dbTable)

	_, err = db.Exec(
		"create clustered index IX_primary on " + fullTable + " (Time, Id);")

	if err != nil {
		log.Fatalln("failed to create index", dbSchema, dbTable, err)
	}
	log.Infoln("created index", dbSchema, dbTable)
}

func configLogger() {
	log.SetFormatter(&log.TextFormatter{ForceColors: true})
	log.SetOutput(colorable.NewColorableStdout())
	log.SetLevel(log.DebugLevel)
}

func main() {
	consumerType := flag.String("consumer", "cli", "( cli | database )")
	connectionString := flag.String("connectionString", "", "<connection-string>")
	dbDriver := flag.String("driver", "mssql", "( mssql | <driver-name> ) -- If the driver name you want is not present, you can provide it, but you need to 'go get' the package yourself on the machine running it")
	dbSchema := flag.String("schema", "dbo", "<db-schema-name>")
	dbTable := flag.String("table", "gatherer", "<db-table-name>")
	dbCreate := flag.Bool("dbCreate", false, "If enabled, creates the database pointed to by the connection string, schema, and table, if it does not exist")

	flag.Parse()

	configLogger()
	g := serve()

	switch *consumerType {
	case "cli":
		consumeCli(g)
	case "database":
		consumeDatabase(g, *dbDriver, *connectionString, *dbSchema, *dbTable, *dbCreate)
	default:
		log.Fatal("unrecognized consumer value ", *consumerType)
	}

}
