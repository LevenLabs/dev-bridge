package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/websocket"

	"github.com/levenlabs/dev-bridge/config"
	"github.com/levenlabs/dev-bridge/router"
	"github.com/levenlabs/go-llog"
	"github.com/levenlabs/go-srvclient"
	"github.com/mediocregopher/skyapi/client"
)

func main() {
	llog.Info("starting dev-bridge")

	if config.SkyAPIAddr != "" {
		skyapiAddr, err := srvclient.SRV(config.SkyAPIAddr)
		if err != nil {
			llog.Fatal("srv lookup of skyapi failed", llog.KV{"err": err})
		}

		kv := llog.KV{"skyapiAddr": skyapiAddr}
		llog.Info("connecting to skyapi", kv)

		go func() {
			kv["err"] = client.ProvideOpts(client.Opts{
				SkyAPIAddr:        skyapiAddr,
				Service:           "dev-bridge",
				ThisAddr:          config.ListenAddr,
				ReconnectAttempts: 3,
			})
			llog.Fatal("skyapi giving up reconnecting", kv)
		}()
	}

	go listenPing()

	kv := llog.KV{"addr": config.ListenAddr}
	llog.Info("listening on http proxy port", kv)
	kv["err"] = http.ListenAndServe(config.ListenAddr, http.HandlerFunc(proxy))
	llog.Fatal("error accepting on http socket", kv)
}

func listenPing() {
	kv := llog.KV{"addr": config.PingAddr}
	llog.Info("listening on udp ping port", kv)
	pa, err := net.ResolveUDPAddr("udp", config.PingAddr)
	if err != nil {
		kv["err"] = err
		llog.Fatal("invalid udp address", kv)
	}

	pc, err := net.ListenUDP("udp", pa)
	if err != nil {
		kv["err"] = err
		llog.Fatal("couldn't listen on udp ping port", kv)
	}

	b := make([]byte, 1024)
	for {
		n, addr, err := pc.ReadFromUDP(b)
		if err != nil {
			kv["err"] = err
			llog.Fatal("error reading from udp port", kv)
		}

		kv["ip"] = addr.IP
		kv["body"] = string(b[:n])
		llog.Debug("udp ping", kv)

		var r router.Route
		if err := json.Unmarshal(b[:n], &r); err != nil {
			kv["err"] = err
			llog.Warn("could not unmarshal json from ping", kv)
			continue
		}

		r.IP = addr.IP
		router.Pinged(r)
	}
}

var reverseProxy = httputil.ReverseProxy{
	// Normally the http proxy director would do something, be we do the request
	// modification beforehand in the proxy function and simply hand-off to
	// reverseProxy for the hard parts, so all of its work is already done
	Director:      func(_ *http.Request) {},
	FlushInterval: 100 * time.Millisecond,
	// Unfortunately httputil.ReverseProxy does not have an option to not log
	// anywhere, so we create a logger to give it which will simply do nothing.
	// TODO figure out a way to log properly using our format
	ErrorLog: log.New(ioutil.Discard, "", 0),
}

func errCouldNotRouteHost(w http.ResponseWriter, kv llog.KV) {
	llog.Warn("could not route", kv)
	http.Error(w, "could not route given host", 400)
}

func proxy(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	kv := llog.KV{
		"host": host,
		"url":  r.URL.String(),
	}
	kv["ip"], _, _ = net.SplitHostPort(r.RemoteAddr)

	llog.Debug("proxy request", kv)

	if host == "" {
		kv["err"] = "no host"
		errCouldNotRouteHost(w, kv)
		return
	}

	rr, ok := router.FindRoute(host, config.WhitelistedSuffixes...)
	if !ok {
		kv["err"] = "no matched route for prefix"
		errCouldNotRouteHost(w, kv)
		return
	}

	fwdAddr := net.JoinHostPort(rr.IP.String(), strconv.Itoa(rr.Port))
	kv["fwdAddr"] = fwdAddr
	r.URL.Host = fwdAddr
	r.Host = strings.TrimPrefix(host, rr.Prefix+".")

	// short-circuit if this is starting a websocket, httputil's reverse proxy
	// doesn't handle websockets
	connHeader := strings.ToLower(r.Header.Get("Connection"))
	upgradeHeader := strings.ToLower(r.Header.Get("Upgrade"))
	if connHeader == "upgrade" && upgradeHeader == "websocket" {
		if rr.HTTPS {
			r.URL.Scheme = "wss"
		} else {
			r.URL.Scheme = "ws"
		}
		llog.Debug("proxy request is websocket", kv)
		proxyWS(w, r, kv)
		return
	}

	if rr.HTTPS {
		r.URL.Scheme = "https"
	} else {
		r.URL.Scheme = "http"
	}

	reverseProxy.ServeHTTP(w, r)
}

func proxyWS(w http.ResponseWriter, r *http.Request, kv llog.KV) {

	// reverseProxy would normally do this for us, but we can't use reverseProxy
	// for websockets :(
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		if xff, ok := r.Header["X-Forwarded-For"]; ok {
			ip = strings.Join(xff, ", ") + ", " + ip
		}
		r.Header.Set("X-Forwarded-For", ip)
	}

	wsconfig, err := websocket.NewConfig(r.URL.String(), r.Header.Get("Origin"))
	if err != nil {
		kv["err"] = err
		llog.Warn("error generating websocket config", kv)
		http.Error(w, "error generating websocket config: "+err.Error(), 500)
		return
	}
	for h, vs := range r.Header {
		wsconfig.Header[h] = vs
	}

	fwdc, err := websocket.DialConfig(wsconfig)
	if err != nil {
		kv["err"] = err
		llog.Warn("error connecting to websocket server", kv)
		http.Error(w, "error connecting to websocket server: "+err.Error(), 500)
		return
	}

	websocket.Handler(func(c *websocket.Conn) {
		go io.Copy(c, fwdc)
		io.Copy(fwdc, c)
	}).ServeHTTP(w, r)
}
