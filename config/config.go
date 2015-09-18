// Package config provides for all configurable parameters dev-bridge can have
package config

import (
	"time"

	"github.com/levenlabs/go-llog"
	"github.com/mediocregopher/lever"
)

// All possible configurable variables
var (
	ListenAddr          string
	PingAddr            string
	SkyAPIAddr          string
	WhitelistedSuffixes []string
	PingTimeout         time.Duration
	LogLevel            string
)

func init() {
	l := lever.New("dev-bridge", nil)
	l.Add(lever.Param{
		Name:        "--listen-addr",
		Description: "address to listen for http connections on",
		Default:     ":8080",
	})
	l.Add(lever.Param{
		Name:        "--ping-addr",
		Description: "udp address that will listen for pings from hosts",
		Default:     ":4445",
	})
	l.Add(lever.Param{
		Name:        "--skyapi-addr",
		Description: "Hostname of skyapi, to be looked up via a SRV request. Unset means don't register with skyapi",
	})
	l.Add(lever.Param{
		Name:        "--whitelist-suffix",
		Description: "Only proxy requests whose Host has this suffix. Can be specified multiple times. If not specified, all Hosts allowed",
	})
	l.Add(lever.Param{
		Name:        "--ping-timeout",
		Description: "Time before a machine must ping again (e.g. 5s, 3m5s)",
		Default:     "30s",
	})
	l.Add(lever.Param{
		Name:        "--log-level",
		Description: "Minimum log level to show, either debug, info, warn, error, or fatal",
		Default:     "info",
	})
	l.Parse()

	ListenAddr, _ = l.ParamStr("--listen-addr")
	PingAddr, _ = l.ParamStr("--ping-addr")
	SkyAPIAddr, _ = l.ParamStr("--skyapi-addr")
	WhitelistedSuffixes, _ = l.ParamStrs("--whitelist-suffix")
	LogLevel, _ = l.ParamStr("--log-level")

	llog.SetLevelFromString(LogLevel)

	pingTimeout, _ := l.ParamStr("--ping-timeout")
	pingTimeoutParsed, err := time.ParseDuration(pingTimeout)
	if err != nil {
		llog.Fatal("invalid ping timeout passed in", llog.KV{
			"pingTimeout": pingTimeout,
			"err":         err,
		})
	}
	PingTimeout = pingTimeoutParsed
}
