package manager

import (
	"fmt"
	"log"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"
	"github.com/samuelngs/axis/health"
	"github.com/samuelngs/axis/launcher"
	"github.com/samuelngs/axis/models"
	"github.com/samuelngs/axis/pkg/network"
)

type (
	// Client - the etcd client
	Client struct {

		// lock
		sync.RWMutex

		// client props
		endpoints []string
		events    chan *models.Event
		cancel    chan struct{}
		client    client.KeysAPI

		// service address and directory
		address string
		dir     *models.Directory

		// service state
		running bool
		started bool
		locked  bool

		// election state
		leader *models.Leader
	}
)

var (
	// ServiceTTL - a period of time after-which the defined service node
	// will be expired and removed from the etcd cluster
	ServiceTTL = time.Second * 10
)

const (
	// DirectoryElection - the path of the election
	DirectoryElection string = "election"
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
	// EventReElected - the leader election is completed
	EventReElected string = "re-elected"
	// EventElection - the leader election is started
	EventElection = "election"
	// EventReady - the service is ready to run
	EventReady = "ready"
	// EventWait - the election wait lock
	EventWait = "wait"

	// GroupLeader - the leader node
	GroupLeader string = "leader"
	// GroupWorker - the worker node
	GroupWorker = "worker"
)

// NewClient - create a etcd client instance
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

// SetupDirectory - setup directory for service
func (c *Client) SetupDirectory() {
	v := reflect.ValueOf(c.dir)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		log.Fatal("only accepts structs")
	}
	for i := 0; i < v.NumField(); i++ {
		key := v.Field(i).String()
		c.client.Set(context.Background(), key, "", &client.SetOptions{
			Dir:       true,
			PrevExist: client.PrevNoExist,
		})
	}
}

// SetDir - set discovery directory
func (c *Client) SetDir(prefix, name string) {
	c.Lock()
	c.dir = &models.Directory{
		Base:     fmt.Sprintf("%v/%v", prefix, name),
		Election: fmt.Sprintf("%v/%v/%v", prefix, name, DirectoryElection),
		Running:  fmt.Sprintf("%v/%v/%v", prefix, name, DirectoryRunning),
		Queue:    fmt.Sprintf("%v/%v/%v", prefix, name, DirectoryQueue),
		Nodes:    fmt.Sprintf("%v/%v/%v", prefix, name, DirectoryNodes),
		Masters:  fmt.Sprintf("%v/%v/%v", prefix, name, DirectoryMasters),
	}
	c.Unlock()
}

// Observe - observe directory
func (c *Client) Observe() {
	// register service
	c.SetupDirectory()
	c.RegisterNode(c.dir.Node(c.address))
	c.RegisterNode(c.dir.QueueNode(c.address))
	c.RegisterNode(c.dir.ElectionNode(c.address))
	// create a interval timer to monitor service nodes
	interval := time.NewTicker(ServiceTTL / 2)
	defer interval.Stop()
	for {
		select {
		case <-interval.C:
			go func() {
				// read running state
				c.RLock()
				var running = c.running
				c.RUnlock()
				// renew nodes
				c.RenewNode(c.dir.Node(c.address))
				c.RenewNode(c.dir.ElectionNode(c.address))
				if running {
					c.RenewNode(c.dir.RunningNode(c.address))
					if c.IsLeader() {
						c.RenewNode(c.dir.MasterNode(c.address))
					}
				} else {
					c.RenewNode(c.dir.QueueNode(c.address))
				}
				c.LeaderDiscovery()
			}()
		}
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
		TTL: ServiceTTL,
	})
	// create watcher
	watcher := c.client.Watcher(c.dir.Election, &client.WatcherOptions{
		AfterIndex: 0,
		Recursive:  true,
	})
	go func() {
		for {
			select {
			case <-c.cancel:
				if !isCancelled {
					cancel()
					isCancelled = true
				}
				return
			}
		}
	}()
	// observe election changes
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
				c.events <- &models.Event{Type: EventElection, Group: GroupWorker}
				go c.LeaderDiscovery()
			}
		}
	}
}

// RegisterNode - register node to etcd
func (c *Client) RegisterNode(dir string) {
	c.client.Set(context.Background(), dir, c.address, &client.SetOptions{
		Dir: false,
		TTL: ServiceTTL,
	})
}

// UnsetNode - unregister node and extend ttl
func (c *Client) UnsetNode(dir string) {
	c.client.Delete(context.Background(), dir, nil)
}

// RenewNode - renew node and extend ttl
func (c *Client) RenewNode(dir string) {
	c.client.Set(context.Background(), dir, c.address, &client.SetOptions{
		PrevExist: client.PrevExist,
		TTL:       ServiceTTL,
	})
}

// RunApplication - run application
func (c *Client) RunApplication(entrypoint *models.ApplicationEntryPoint) {
	c.RLock()
	if c.started {
		return
	}
	c.RUnlock()
	c.Lock()
	c.started = true
	c.Unlock()
	receive := make(chan string)
	// generate scope
	scope := c.GenerateScope()
	// launcher start daemon
	go launcher.Start(scope, entrypoint)
	// health check
	if entrypoint.Health != nil && entrypoint.Health.Ports != nil {
		go health.Check(receive, entrypoint.Health.Ports...)
	}
	for {
		select {
		case event := <-receive:
			switch event {
			case health.Pass:
				c.Lock()
				if !c.running {
					fmt.Println("service is now running")
				}
				c.running = true
				c.Unlock()
				c.RegisterNode(c.dir.RunningNode(c.address))
				if c.IsLeader() {
					c.RegisterNode(c.dir.MasterNode(c.address))
				}
			case health.Fail:
				c.Lock()
				if c.running {
					fmt.Println("service is now stopped")
				}
				c.running = false
				c.Unlock()
				c.UnsetNode(c.dir.RunningNode(c.address))
			}
		}
	}
}

// IsLeader - is current node a leader
func (c *Client) IsLeader() bool {
	// self node key
	self := c.dir.ElectionNode(c.address)
	if c.leader != nil && c.leader.Key == self {
		return true
	}
	return false
}

// LeaderDiscovery - get leader/master node information
func (c *Client) LeaderDiscovery() {
	dir := c.dir.Election
	// self node key
	self := fmt.Sprintf("%v/%v", dir, c.address)
	// get a list of election nodes
	resp, err := c.client.Get(context.Background(), dir, &client.GetOptions{Sort: true})
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
		if c.leader == nil {
			if leader.Key == self {
				fmt.Println("# elected as leader")
				c.events <- &models.Event{Type: EventElected, Group: GroupLeader}
			} else {
				fmt.Println("# elected as worker")
				// do not send any event until leader node is ready
				if nodes := c.GetRunningNodes(); len(nodes) > 0 {
					c.events <- &models.Event{Type: EventElected, Group: GroupWorker}
				} else {
					go c.WaitForLeader()
				}
			}
		} else if c.leader != nil && leader.Key != c.leader.Key {
			if leader.Key == self {
				fmt.Println("# re-elected as leader")
				c.events <- &models.Event{Type: EventReElected, Group: GroupLeader}
			}
		}
		c.Lock()
		c.leader = leader
		c.Unlock()
	}
}

// WaitForLeader - wait for leader node is ready
func (c *Client) WaitForLeader() {
	defer func() {
		c.Lock()
		c.locked = false
		c.Unlock()
	}()
	c.RLock()
	var locked = c.locked
	c.RUnlock()
	if !locked {
		fmt.Println("# waiting for leader node")
		c.Lock()
		c.locked = true
		c.Unlock()
		interval := time.NewTicker(ServiceTTL)
		defer interval.Stop()
		for {
			select {
			case <-interval.C:
				if c.IsLeader() {
					return
				}
				fmt.Println("# scanning running nodes...")
				if nodes := c.GetRunningNodes(); len(nodes) > 0 {
					c.events <- &models.Event{Type: EventElected, Group: GroupWorker}
				} else {
					fmt.Println("# no nodes are ready yet")
				}
			}
		}
	}
}

// GenerateScope - generate scope base
func (c *Client) GenerateScope() *models.Scope {
	return models.SetupEnvironment(
		c.GetServiceHostname(),
		c.GetServiceIP(),
		c.GetRunningNodes(),
	)
}

// GetRunningNodes to get existed nodes
func (c *Client) GetRunningNodes() []models.Node {
	dir := c.dir.Running
	res := []models.Node{}
	if c.client == nil {
		return res
	}
	resp, err := c.client.Get(context.Background(), dir, nil)
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
	for i := 0; i < 3; i++ {
		switch i {
		case 0:
			if c.endpoints != nil && len(c.endpoints) > 0 {
				return c.endpoints
			}
		case 1:
			env := c.GetEnvEndPoint()
			if strings.TrimSpace(env) == "" {
				continue
			}
			if arr := strings.Split(env, ","); len(arr) > 0 {
				return arr
			}
		case 2:
			addr := c.GetServiceHostIP()
			return []string{
				fmt.Sprintf("http://%v:2379", addr),
				fmt.Sprintf("http://%v:4001", addr),
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

// GetServiceHostIP - return service host ip (container host)
func (c *Client) GetServiceHostIP() string {
	output, err := network.IP("route")
	if err != nil {
		log.Fatal(err)
	}
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "default") {
			continue
		}
		parts := strings.Split(line, " ")
		for _, part := range parts {
			if ip := net.ParseIP(part); ip != nil {
				return part
			}
		}
	}
	return ""
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
	fmt.Printf("# connect to %v\n", endpoints)
	conn, err := client.New(cfg)
	if err != nil {
		return err
	}
	kapi := client.NewKeysAPI(conn)
	c.client = kapi
	return nil
}
