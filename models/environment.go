package models

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"reflect"
	"strings"
)

// Scope - the environment scope
type Scope struct {
	Hostname string
	IP       string
	Group    string
	Nodes    Nodes
}

// SetupEnvironment - to create a scope instance
func SetupEnvironment(hostname, ip, group string, nodes Nodes) *Scope {
	return &Scope{
		Hostname: hostname,
		IP:       ip,
		Group:    group,
		Nodes:    nodes,
	}
}

// Compile - to compile instructions with variables
func (scope Scope) Compile(cmd string) string {
	// create doc buffer
	var doc bytes.Buffer
	// create map
	variables := make(map[string]interface{})
	v := reflect.ValueOf(scope)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		log.Fatal("only accepts structs")
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		row := t.Field(i)
		key := strings.ToUpper(row.Name)
		val := v.Field(i).Interface()
		variables[fmt.Sprintf("AXIS_%v", key)] = val
	}
	tmpl, err := template.New("cmd").Parse(cmd)
	if err != nil {
		log.Fatal(err)
	}
	err = tmpl.Execute(&doc, variables)
	if err != nil {
		log.Fatal(err)
	}
	return doc.String()
}
