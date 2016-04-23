package main

import (
	"log"
	"os"

	"github.com/samuelngs/axis/manager"
	"github.com/samuelngs/axis/models"
	"github.com/samuelngs/axis/parser"
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
	client := manager.NewClient(conf.Etcd.Endpoints)

	// connect etcd server
	if err := client.Connect(); err != nil {
		log.Fatal("unable connect etcd client")
	}

	// set etcd service directory
	client.SetDir(
		conf.Daemon.Prefix, // <= prefix name
		conf.Daemon.Name,   // <= service name
	)

	// listen to election events
	go process(client, conf)

	// observe service directory
	go client.Observe()

	// execute etcd `leader` election
	client.Election()
}

func process(client *manager.Client, conf *models.YamlOptions) {
	for {
		select {
		case event := <-client.Events():
			switch event.Type {
			case manager.EventElected:
				switch event.Group {
				case manager.GroupLeader:
					client.RunApplication(conf.Daemon.Leader)
				case manager.GroupWorker:
					client.RunApplication(conf.Daemon.Worker)
				}
			}
		}
	}
}
