# dev-bridge

An http(s) proxy which accepts incoming connections and, depending on their
Host, directs them to a particular other machine. Machines register themselves
using periodic UDP pings.

dev-bridge supports websockets as well.

## Purpose

The purpose of dev-bridge is, like the name implies, to create a bridge between
the outer world and dev machines on a vpn. In this way developers can simply
have their environment connected to the vpn and have it accessible to others,
even if the others' device isn't on the vpn (which makes mobile testing
significantly easier), all without anyone mucking around with NATs and port
forwarding and such.

## Example

Machine Foo sends the following UDP packet periodically to dev-bridge over the
vpn:

```
{
    "prefix":"foo",
    "port":80,
    "https":false
}
```

dev-bridge, which is accepting on all interfaces, will look at every http
request it receives. If any have a `Host` which has the prefix `foo.` then they
will be forwarded to the ip that Foo was sending packets from, on port 80,
without https, and with the `foo.` prefix stripped from the `Host` header.

## Other features

* Ability to whitelist `Host` suffixes
* Adds X-Forwarded-For as requests pass through
