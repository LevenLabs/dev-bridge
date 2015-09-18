package main

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	. "testing"

	"github.com/levenlabs/dev-bridge/router"
	"github.com/levenlabs/golib/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testHost pretends to be a test dev machine which has pinged as some random
// string. For every request that hits it the test server will send the
// http.Request object to the channel which is returned. It will also echo the
// body it receives as an actual response to the request. The returned string is
// the prefix it has registered as
func testHost() (string, chan *http.Request) {
	prefix := testutil.RandStr()
	ch := make(chan *http.Request, 1)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Bar", "BAR")
			ch <- r
			io.Copy(w, r.Body)
		},
	))

	ipStr, portStr, err := net.SplitHostPort(server.URL[7:])
	if err != nil {
		panic(err)
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		panic("couldn't parse ip from httptest: " + ipStr)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(err)
	}

	router.Pinged(router.Route{
		Prefix: prefix,
		Port:   port,
		HTTPS:  false,
		IP:     ip,
	})

	return prefix, ch
}

func TestProxy(t *T) {
	prefix, ch := testHost()

	hostSuffix := testutil.RandStr()
	host := prefix + "." + hostSuffix
	r, err := http.NewRequest("GET", "http://"+host, bytes.NewBufferString("OHAI"))
	require.Nil(t, err)

	r.Header.Set("X-Foo", "FOO")

	resp := httptest.NewRecorder()
	proxy(resp, r)

	rr := <-ch
	assert.Equal(t, hostSuffix, rr.Host)
	assert.Equal(t, "FOO", rr.Header.Get("X-Foo"))
	assert.Equal(t, "OHAI", resp.Body.String())
	assert.Equal(t, "BAR", resp.Header().Get("X-Bar"))
}

// Looks like reverse proxy does this for us. sweet!
func TestXForwardedFor(t *T) {
	// Test with no previously set X-Forwarded-For
	prefix, ch := testHost()
	hostSuffix := testutil.RandStr()
	host := prefix + "." + hostSuffix
	r, err := http.NewRequest("GET", "http://"+host, bytes.NewBufferString("OHAI"))
	require.Nil(t, err)
	r.RemoteAddr = "127.0.0.1:666"

	resp := httptest.NewRecorder()
	proxy(resp, r)
	rr := <-ch
	assert.Equal(t, "127.0.0.1", rr.Header.Get("X-Forwarded-For"))

	// Test with an already set X-Forwarded-For
	prefix, ch = testHost()
	hostSuffix = testutil.RandStr()
	host = prefix + "." + hostSuffix
	r, err = http.NewRequest("GET", "http://"+host, bytes.NewBufferString("OHAI"))
	require.Nil(t, err)
	r.RemoteAddr = "127.0.0.1:666"
	r.Header.Set("X-Forwarded-For", "8.8.8.8")

	resp = httptest.NewRecorder()
	proxy(resp, r)
	rr = <-ch
	assert.Equal(t, "8.8.8.8, 127.0.0.1", rr.Header.Get("X-Forwarded-For"))
}
