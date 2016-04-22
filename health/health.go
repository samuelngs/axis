package health

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	// Pass - Result Passed
	Pass string = "OK"
	// Fail - Result Failed
	Fail = "FAIL"
)

var (
	checkInterval = time.Second * 5
)

// Check - monitor if ports are open or close
func Check(receive chan string, ports ...string) {
	if len(ports) == 0 {
		receive <- Pass
	}
	// create a timer to check services health
	timer := time.NewTicker(checkInterval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			res := Pass
			for _, i := range ports {
				p := strings.Split(i, "/")
				l := len(p)
				if l <= 0 && l >= 3 {
					log.Fatal("port format is invalid")
				}
				if _, err := strconv.Atoi(p[0]); err != nil {
					log.Fatal("port number is invalid")
				}
				var protocol, port string
				if l == 2 {
					protocol = p[1]
				} else {
					protocol = "tcp"
				}
				port = p[0]
				c, err := net.ListenPacket(protocol, fmt.Sprintf(":%v", port))
				defer c.Close()
				if err != nil {
					res = Fail
					break
				}
			}
			receive <- res
		}
	}
}
