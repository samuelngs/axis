package models

import "fmt"

type (
	// Node - the service node
	Node string
	// Nodes - the service nodes
	Nodes []Node
	// Leader - the leader node
	Leader struct {
		Key     string
		Address string
	}
	// Directory - the node directory
	Directory struct {
		Base     string
		Election string
		Running  string
		Queue    string
		Nodes    string
		Masters  string
	}
)

// ElectionNode - generate election node address
func (dir *Directory) ElectionNode(addr string) string {
	return fmt.Sprintf("%v/%v", dir.Election, addr)
}

// RunningNode - generate running node address
func (dir *Directory) RunningNode(addr string) string {
	return fmt.Sprintf("%v/%v", dir.Running, addr)
}

// Node - generate node address
func (dir *Directory) Node(addr string) string {
	return fmt.Sprintf("%v/%v", dir.Nodes, addr)
}
