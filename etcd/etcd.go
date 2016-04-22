package etcd

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"
	"github.com/samuelngs/axis/models"
)

type (
	// Client - the etcd client
	Client struct {

		// client props
		endpoints []string
		events    chan *models.Event
		cancel    chan struct{}
		client    client.KeysAPI

		// service address and directory
		address string
		dir     *models.Directory

		// election state
		sync.RWMutex
		leader *models.Leader
	}
)

var (
	// ElectionTTL - a period of time after-which the defined election node
	// will be expired and removed from the etcd cluster
	ElectionTTL = time.Second * 10
)

const (
	// DirectoryLock - the path of the lock
	DirectoryLock string = "lock"
	// DirectoryElection - the path of the election
	DirectoryElection = "election"
	// DirectoryMasters - the path of the masters
	DirectoryMasters = "masters"
	// DirectoryNodes - the path of the nodes
	DirectoryNodes = "nodes"
	// DirectoryRunning - the list of running nodes
	DirectoryRunning = "running"
	// DirectoryQueue - the list of starting nodes in queue
	DirectoryQueue = "queue"

	// EventElected - the leader election is completed
	EventElected string = "elected"
	// EventElecting - the leader election is currently processing
	EventElecting = "electing"
	// EventDead - the leader is dead
	EventDead = "dead"
	// EventLock - the election lock is ready
	EventLock = "lock"
	// EventWait - the election wait lock
	EventWait = "wait"

	// GroupLeader - the leader node
	GroupLeader string = "leader"
	// GroupWorker - the worker node
	GroupWorker = "worker"
)

// NewClient - create a etcd clien instance
func NewClient(endpoints ...[]string) *Client {
	client := &Client{
		events: make(chan *models.Event),
		cancel: make(chan struct{}),
	}
	for _, v := range endpoints {
		client.endpoints = v
	}
	client.address = client.GetServiceIP()
	return client
}

// Events - the etcd events
func (c *Client) Events() chan *models.Event {
	return c.events
}

// Leader - the leader node
func (c *Client) Leader() *models.Leader {
	var leader *models.Leader
	c.RLock()
	leader = c.leader
	c.RUnlock()
	return leader
}

// SetDirectory - set directory for the etcd watcher
func (c *Client) SetDirectory(prefix, name string) {
	c.dir = &models.Directory{
		Base:     fmt.Sprintf("%v/%v", prefix, name),
		Election: fmt.Sprintf("%v/%v/%v", prefix, name, DirectoryElection),
		Running:  fmt.Sprintf("%v/%v/%v", prefix, name, DirectoryRunning),
		Queue:    fmt.Sprintf("%v/%v/%v", prefix, name, DirectoryQueue),
		Nodes:    fmt.Sprintf("%v/%v/%v", prefix, name, DirectoryNodes),
		Masters:  fmt.Sprintf("%v/%v/%v", prefix, name, DirectoryMasters),
	}
}

// Election - to start leader election task
func (c *Client) Election() {
	defer func() {
		// recover if panic
		if r := recover(); r != nil {
			c.Election()
		}
	}()
	// determine if context is already cancelled
	isCancelled := false
	// create context with cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if !isCancelled {
			cancel()
			isCancelled = true
		}
	}()
	// generate election key
	key := c.dir.ElectionNode(c.address)
	// create election directory if it does not exist
	c.client.Set(ctx, key, c.address, &client.SetOptions{
		Dir: false,
		TTL: ElectionTTL,
	})
	// create a timer to refresh the etcd node
	refresh := time.NewTicker(ElectionTTL / 2)
	defer refresh.Stop()
	// observe election changes
	go c.Observe(ctx)
	for {
		select {
		case <-refresh.C:
			c.ExtendTTL(ctx)
			c.LookupLeader(ctx)
		case <-c.cancel:
			if !isCancelled {
				cancel()
				isCancelled = true
			}
			return
		}
	}
}

// ExtendTTL - to node extend ttl
func (c *Client) ExtendTTL(ctx context.Context) {
	// generate election key
	key := c.dir.ElectionNode(c.address)
	// extend ttl
	c.client.Set(ctx, key, c.address, &client.SetOptions{
		PrevExist: client.PrevExist,
		TTL:       ElectionTTL,
	})
}

// Observe - to observe the etcd nodes changes
func (c *Client) Observe(ctx context.Context) {
	// create watcher
	watcher := c.client.Watcher(c.dir.Election, &client.WatcherOptions{
		AfterIndex: 0,
		Recursive:  true,
	})
	for {
		resp, err := watcher.Next(ctx)
		if err != nil {
			panic(err)
		}
		if resp.Node.Dir {
			continue
		}
		if c.Leader() == nil {
			continue
		}
		switch resp.Action {
		case "set", "update":
		case "delete":
			if leader := c.Leader(); leader.Key == resp.Node.Key {
				c.events <- &models.Event{Type: EventDead, Group: GroupWorker}
				c.events <- &models.Event{Type: EventElecting, Group: GroupWorker}
				c.LookupLeader(ctx)
			}
		}
	}
}

// LookupLeader - get leader/master node information
func (c *Client) LookupLeader(ctx context.Context) {
	dir := c.dir.Election
	// self node key
	self := fmt.Sprintf("%v/%v", dir, c.address)
	// get a list of election nodes
	resp, err := c.client.Get(ctx, dir, &client.GetOptions{Sort: true})
	if err != nil {
		log.Fatal(err)
	}
	// leader key and address
	var key, addr string
	// current lowest node index
	var idx uint64
	if len(resp.Node.Nodes) > 0 {
		for _, v := range resp.Node.Nodes {
			if v.Dir {
				continue
			}
			if idx == 0 || v.CreatedIndex < idx {
				key = v.Key
				addr = v.Value
				idx = v.CreatedIndex
			}
		}
	}
	if key == "" || addr == "" {
		fmt.Println("# no nodes were found")
		c.Lock()
		c.leader = nil
		c.Unlock()
	} else {
		leader := &models.Leader{Key: key, Address: addr}
		if c.leader == nil && leader.Key == self {
			fmt.Println("# elected as leader")
			c.events <- &models.Event{Type: EventElected, Group: GroupLeader, Scope: c.GenerateScope(ctx, GroupLeader)}
		} else if c.leader != nil && leader.Key != c.leader.Key {
			if leader.Key == self {
				fmt.Println("# elected as leader")
				c.events <- &models.Event{Type: EventElected, Group: GroupLeader, Scope: c.GenerateScope(ctx, GroupLeader)}
			} else {
				fmt.Println("# elected as worker")
				c.events <- &models.Event{Type: EventElected, Group: GroupWorker, Scope: c.GenerateScope(ctx, GroupWorker)}
			}
		}
		c.Lock()
		c.leader = leader
		c.Unlock()
	}
}

// GenerateScope - generate scope base
func (c *Client) GenerateScope(ctx context.Context, group string) *models.Scope {
	return models.SetupEnvironment(
		c.GetServiceHostname(),
		c.GetServiceIP(),
		group,
		c.GetRunningNodes(ctx),
	)
}

// GetRunningNodes to get existed nodes
func (c *Client) GetRunningNodes(ctx context.Context) []models.Node {
	dir := c.dir.Running
	res := []models.Node{}
	if c.client == nil {
		return res
	}
	resp, err := c.client.Get(ctx, dir, nil)
	if err != nil {
		return res
	}
	if !resp.Node.Dir {
		return res
	}
	for _, node := range resp.Node.Nodes {
		res = append(res, models.Node(node.Value))
	}
	return res
}

// GetEnvEndPoint - to extract etcd endpoint environment from shell
func (c *Client) GetEnvEndPoint() string {
	whitelist := []string{"ETCD_ENDPOINT", "ETCDCTL_ENDPOINT", "ETCD_HOST", "COREOS_PRIVATE_IPV4", "COREOS_PUBLIC_IPV4"}
	for _, i := range whitelist {
		if v := os.Getenv(i); v != "" {
			return v
		}
	}
	return ""
}

// GetEndPoint - to get endpoint from config, env or docker host
func (c *Client) GetEndPoint() []string {
	for i := 0; i < 2; i++ {
		switch i {
		case 0:
			if c.endpoints != nil && len(c.endpoints) > 0 {
				return c.endpoints
			}
		case 1:
			if arr := strings.Split(c.GetEnvEndPoint(), ","); len(arr) > 0 {
				return arr
			}
		}
	}
	return []string{"http://127.0.0.1:2379", "http://127.0.0.1:4001"}
}

// GetServiceHostname - extract FQDN hostname from kernel
func (c *Client) GetServiceHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return hostname
}

// GetServiceIP - get service ip address
func (c *Client) GetServiceIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return ""
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String()
		}
	}
	return ""
}

// Connect to connect etcd client
func (c *Client) Connect() error {
	endpoints := c.GetEndPoint()
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
