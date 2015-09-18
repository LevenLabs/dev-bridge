// Package router deals solely with managing the mapping from host prefix to
// routing information for requests directed there
package router

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/levenlabs/dev-bridge/config"
	"github.com/levenlabs/go-llog"
)

// Route describes a single endpoint which is accepting requests for the Route's
// prefix
type Route struct {
	Prefix   string `json:"prefix"`
	Port     int    `json:"port"`
	HTTPS    bool   `json:"https"`
	IP       net.IP `json:"-"`
	lastPing time.Time
}

var routes = map[string]Route{}
var routesL = sync.RWMutex{}

func init() {
	go func() {
		for range time.Tick(config.PingTimeout / 2) {
			cleanRoutes(config.PingTimeout)
		}
	}()
}

// Pinged adds or updates the given Route's information in the routing table
func Pinged(r Route) {
	r.lastPing = time.Now()
	routesL.Lock()
	defer routesL.Unlock()
	routes[r.Prefix] = r
}

// FindRoute returns an available Route for a request directed at the given
// host, or false if none is found. If any whitelistedSuffixes are given then
// host must have one of them, or no Route will be returned
func FindRoute(host string, whitelistedSuffixes ...string) (Route, bool) {

	if len(whitelistedSuffixes) > 0 {
		var found bool
		for _, s := range whitelistedSuffixes {
			if s[0] != '.' {
				s = "." + s
			}
			if strings.HasSuffix(host, s) {
				found = true
				break
			}
		}
		if !found {
			return Route{}, false
		}
	}

	prefix := strings.SplitN(host, ".", 2)[0]
	routesL.RLock()
	defer routesL.RUnlock()
	r, ok := routes[prefix]
	return r, ok
}

func cleanRoutes(timeout time.Duration) {
	routesL.Lock()
	defer routesL.Unlock()
	for prefix, r := range routes {
		if time.Since(r.lastPing) > timeout {
			llog.Warn("route timed out", llog.KV{"prefix": r.Prefix})
			delete(routes, prefix)
		}
	}
}
