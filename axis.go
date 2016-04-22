package main

import (
	"log"
	"os"

	"github.com/samuelngs/axis/etcd"
	"github.com/samuelngs/axis/health"
	"github.com/samuelngs/axis/launcher"
	"github.com/samuelngs/axis/models"
	"github.com/samuelngs/axis/parser"
)

var (
	daemonStarted = false
	healthStarted = false
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
	go process(client, conf.Daemon)

	// set etcd service directory
	client.SetDirectory(
		conf.Daemon.Prefix, // <= prefix name
		conf.Daemon.Name,   // <= service name
	)

	// execute etcd `leader` election
	client.Election()
}

func process(client *etcd.Client, opts *models.ApplicationOptions) {
	for {
		select {
		case event := <-client.Events():
			switch event.Type {
			case etcd.EventElected:
				go launchHealthCheck(client, event.Scope, opts)
				go launchApplication(client, event.Scope, opts)
			case etcd.EventElecting:
			case etcd.EventDead:
			}
		}
	}
}

func launchApplication(client *etcd.Client, scope *models.Scope, opts *models.ApplicationOptions) {
	if daemonStarted {
		return
	}
	defer func() {
		daemonStarted = false
	}()
	daemonStarted = true
	close := make(chan struct{}, 1)
	switch scope.Group {
	case etcd.GroupLeader:
		go launcher.Start(close, scope, opts.Leader)
	case etcd.GroupWorker:
		go launcher.Start(close, scope, opts.Worker)
	}
	<-close
	os.Exit(0)
}

func launchHealthCheck(client *etcd.Client, scope *models.Scope, opts *models.ApplicationOptions) {
	if healthStarted {
		return
	}
	defer func() {
		healthStarted = false
	}()
	healthStarted = true
	receive := make(chan string)
	switch scope.Group {
	case etcd.GroupLeader:
		go health.Check(receive, opts.Leader.Health.Ports...)
	case etcd.GroupWorker:
		go health.Check(receive, opts.Worker.Health.Ports...)
	}
	for {
		select {
		case msg := <-receive:
			switch msg {
			case health.Pass:
				client.SetServiceRunning()
			case health.Fail:
				client.UnsetServiceRunning()
			}
		}
	}
}
