package etcd

import (
	"encoding/json"
	"errors"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"
)

type (
	// Client is the etcd client
	Client struct {
		Endpoints []string
		client    client.KeysAPI
	}
	// Node is the service node
	Node struct {
		IP   string `json:"IP"`
		Port int    `json:"PORT"`
	}
)

// IsConnected to check if connection has been established
func (c *Client) IsConnected() bool {
	return c.client != nil
}

// Connect to connect etcd client
func (c *Client) Connect() error {
	endpoints := []string{"http://127.0.0.1:2379"}
	if c.Endpoints != nil {
		endpoints = c.Endpoints
	}
	cfg := client.Config{
		Endpoints:               endpoints,
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
	}
	conn, err := client.New(cfg)
	if err != nil {
		return err
	}
	kapi := client.NewKeysAPI(conn)
	c.client = kapi
	return nil
}

// parseNode to read node value
func (c *Client) parseNode(str string) (*Node, error) {
	var node *Node
	err := json.Unmarshal([]byte(str), &node)
	if err != nil {
		return nil, err
	}
	return node, nil
}

// GetNodes to get existed nodes
func (c *Client) GetNodes(discoveryURL string) ([]*Node, error) {
	res := []*Node{}
	if !c.IsConnected() {
		return nil, errors.New("client is not connected")
	}
	resp, err := c.client.Get(context.Background(), discoveryURL, nil)
	if err != nil {
		return nil, err
	}
	node := resp.Node
	if !node.Dir {
		return nil, errors.New("discovery url only accept a directory")
	}
	nodes := node.Nodes
	for _, n := range nodes {
		if v, err := c.parseNode(n.Value); err == nil {
			res = append(res, v)
		} else {
			return nil, err
		}
	}
	return res, nil
}
