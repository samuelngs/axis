package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/samuelngs/atlas/etcd"
	"github.com/samuelngs/atlas/parser"
)

func main() {

	// get program arguments
	args := os.Args[1:]

	var filename string
	// count args
	if l := len(args); l == 0 {
		filename = "atlas.yaml"
	} else if l > 1 {
		filename = args[0]
	}

	data, ioErr := ioutil.ReadFile(filename)

	if ioErr != nil {
		fmt.Println("unable read configuration file")
		return
	}

	opts, parseErr := parser.ParseYaml(data)

	if parseErr != nil {
		fmt.Println("unable parse configuration file")
		return
	}

	if opts.Daemon.EntryPoint == "" {
		fmt.Println("require entrypoint")
		return
	}

	if opts.Daemon.Master == nil {
		fmt.Println("require master command")
		return
	}

	if opts.Daemon.Slave == nil {
		fmt.Println("require slave command")
		return
	}

	client := &etcd.Client{
		Endpoints: opts.Etcd.Endpoints,
	}

	if err := client.Connect(); err != nil {
		fmt.Println("unable connect etcd client")
		return
	}

	// Get all exists nodes
	nodes, err := client.GetNodes(opts.Daemon.Discovery)

	if err != nil {
		fmt.Println("unable fetch existed service nodes")
		return
	}

	// build commands
	var execute []string
	commands := []string{}

	count := len(nodes)

	if count > 0 {
		execute = opts.Daemon.Slave
	} else {
		execute = opts.Daemon.Master
	}

	for _, c := range execute {
		var str string
		if count > 0 {
			var doc bytes.Buffer
			tmpl, err := template.New("node").Parse(c)
			if err != nil {
				panic(err)
			}
			err = tmpl.Execute(&doc, nodes)
			if err != nil {
				panic(err)
			}
			str = doc.String()
		} else {
			str = c
		}
		if str != "" {
			commands = append(commands, str)
		}
	}

	if err := exec.Command(opts.Daemon.EntryPoint, commands...).Run(); err != nil {
		fmt.Println(err)
		return
	}
}
