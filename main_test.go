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
	"golang.org/x/net/websocket"
)

// testHost pretends to be a test dev machine which has pinged as some random
// string. For every request that hits it the test server will send the
// http.Request object to the channel which is returned. It will also echo the
// body it receives as an actual response to the request. The returned string is
// the prefix it has registered as
func testHost() (string, chan *http.Request) {
	prefix := testutil.RandStr()
	ch := make(chan *http.Request, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Bar", "BAR")
		ch <- r
		websocket.Handler(func(c *websocket.Conn) {
			io.Copy(c, c)
		}).ServeHTTP(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Bar", "BAR")
		ch <- r
		io.Copy(w, r.Body)
	})
	server := httptest.NewServer(mux)

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

func TestWebsockets(t *T) {
	prefix, ch := testHost()
	hostSuffix := testutil.RandStr()
	host := prefix + "." + hostSuffix

	devBridgeServer := httptest.NewServer(http.HandlerFunc(proxy))
	c, err := net.Dial("tcp", devBridgeServer.URL[7:])
	require.Nil(t, err)

	wsconfig, err := websocket.NewConfig("ws://"+host+"/ws", "http://"+host+"/")
	require.Nil(t, err)

	wsc, err := websocket.NewClient(wsconfig, c)
	require.Nil(t, err)

	rr := <-ch
	assert.Equal(t, "127.0.0.1", rr.Header.Get("X-Forwarded-For"))

	_, err = wsc.Write([]byte("ohai"))
	require.Nil(t, err)
	b := make([]byte, 10)
	n, err := wsc.Read(b)
	require.Nil(t, err)
	assert.Equal(t, []byte("ohai"), b[:n])

	// Do it again, sending an X-Forwarded-For
	wsconfig.Header.Set("X-Forwarded-For", "8.8.8.8")
	c, err = net.Dial("tcp", devBridgeServer.URL[7:])
	require.Nil(t, err)
	wsc, err = websocket.NewClient(wsconfig, c)
	require.Nil(t, err)

	rr = <-ch
	assert.Equal(t, "8.8.8.8, 127.0.0.1", rr.Header.Get("X-Forwarded-For"))
}
