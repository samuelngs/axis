package main

import (
	"log"
	"os"

	"github.com/samuelngs/axis/etcd"
	"github.com/samuelngs/axis/launcher"
	"github.com/samuelngs/axis/models"
	"github.com/samuelngs/axis/parser"
)

var (
	daemonStarted = false
)

func main() {

	// get program arguments
	args := os.Args[1:]

	// extract filename from arguments
	var filename string
	switch {
	case len(args) == 0:
		filename = "axis.yaml"
	case len(args) == 1:
		filename = args[0]
	default:
		log.Fatal("only allow a single configuration file")
		return
	}

	// open yaml file and parse the configurations
	conf, err := parser.OpenYaml(filename)
	if err != nil {
		log.Fatal(err)
	}

	// create etcd client
	client := etcd.NewClient(conf.Etcd.Endpoints)

	// connect etcd server
	if err := client.Connect(); err != nil {
		log.Fatal("unable connect etcd client")
	}

	// process election events
	go process(client.Events(), conf.Daemon)

	// set etcd service directory
	client.SetDirectory(
		conf.Daemon.Prefix, // <= prefix name
		conf.Daemon.Name,   // <= service name
	)

	// execute etcd `leader` election
	client.Election()
}

func process(receive chan *models.Event, opts *models.ApplicationOptions) {
	for {
		select {
		case event := <-receive:
			switch event.Type {
			case etcd.EventElected:
				go launchApplication(event.Scope, opts)
			case etcd.EventElecting:
			case etcd.EventDead:
			}
		}
	}
}

func launchApplication(scope *models.Scope, opts *models.ApplicationOptions) {
	if daemonStarted {
		return
	}
	defer func() {
		daemonStarted = false
	}()
	daemonStarted = true
	switch scope.Group {
	case etcd.GroupLeader:
		launcher.Start(scope, opts.Leader)
	case etcd.GroupWorker:
		launcher.Start(scope, opts.Worker)
	}
}
