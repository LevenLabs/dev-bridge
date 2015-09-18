package router

import (
	"net"
	. "testing"
	"time"

	"github.com/levenlabs/golib/testutil"
	"github.com/stretchr/testify/assert"
)

func randRoute() Route {
	return Route{
		Prefix: testutil.RandStr(),
		Port:   10000 + int(testutil.RandInt64()%30000),
		HTTPS:  false,
		IP:     net.ParseIP("127.0.0.1"),
	}
}

func TestBasic(t *T) {
	r := randRoute()
	Pinged(r)

	r2, ok := FindRoute(r.Prefix + "." + testutil.RandStr())
	assert.True(t, ok)
	// r2 will have lastPing updated, but r won't. So we check that separately
	assert.False(t, r2.lastPing.IsZero())
	r2.lastPing = time.Time{}
	assert.Equal(t, r, r2)

	// Test not finding a Route
	_, ok = FindRoute(testutil.RandStr() + "." + testutil.RandStr())
	assert.False(t, ok)
}

func TestWhitelist(t *T) {
	suffix := testutil.RandStr()

	r := randRoute()
	Pinged(r)

	host := r.Prefix + "." + suffix
	r2, ok := FindRoute(host, suffix)
	assert.True(t, ok)
	r2.lastPing = time.Time{}
	assert.Equal(t, r, r2)

	host = r.Prefix + "." + testutil.RandStr() + "." + suffix
	r2, ok = FindRoute(host, suffix)
	assert.True(t, ok)
	r2.lastPing = time.Time{}
	assert.Equal(t, r, r2)

	host = r.Prefix + "." + testutil.RandStr()
	_, ok = FindRoute(host, suffix)
	assert.False(t, ok)
}

func TestClean(t *T) {
	r := randRoute()
	Pinged(r)

	// First clean immediately to show that routes within the timeout won't be
	// removed
	cleanRoutes(1 * time.Second)
	_, ok := FindRoute(r.Prefix + "." + testutil.RandStr())
	assert.True(t, ok)

	// hack the ping time a bit, then do it again
	routesL.Lock()
	r.lastPing = time.Now().Add(-2 * time.Second)
	routes[r.Prefix] = r
	routesL.Unlock()

	cleanRoutes(1 * time.Second)
	_, ok = FindRoute(r.Prefix + "." + testutil.RandStr())
	assert.False(t, ok)
}
