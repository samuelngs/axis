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
	Pass string = "PASS"
	// Fail - Result Failed
	Fail = "FAIL"
)

var (
	checkInterval = time.Second * 5
)

// Check - monitor if ports are open or close
func Check(receive chan string, ports ...string) {
	// create a timer to check services health
	timer := time.NewTicker(checkInterval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			if len(ports) == 0 {
				receive <- Pass
			} else {
				res := Pass
			checks:
				for _, i := range ports {
					if res == Fail {
						continue checks
					}
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
					c, err := net.Dial(protocol, fmt.Sprintf("0.0.0.0:%v", port))
					if err != nil {
						fmt.Printf("health check [%v/%v] error: %v\n", port, protocol, err)
						res = Fail
						continue checks
					}
					defer c.Close()
				}
				receive <- res
			}
		}
	}
}
