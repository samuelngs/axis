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
	} else if l == 1 {
		filename = args[0]
	} else {
		fmt.Println("only allow a single configuration file")
		return
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

	if ipv4 := os.Getenv("COREOS_PRIVATE_IPV4"); ipv4 != "" {
		endpoint := fmt.Sprintf("http://%v:4001", ipv4)
		opts.Etcd.Endpoints = []string{endpoint}
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
		fmt.Println("unable fetch existed service nodes:", err)
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
		if len(c) > 0 && string(c[0]) == "$" {
			str = os.Getenv(c[1:len(c)])
		} else if count > 0 {
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

	c := make(chan error)

	go func() {
		cmd := exec.Command(opts.Daemon.EntryPoint, commands...)
		defer func() {
			cmd.Process.Kill()
		}()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			panic(err)
		}
		c <- cmd.Wait()
	}()

	if err := <-c; err != nil {
		fmt.Println(err)
	}
}
