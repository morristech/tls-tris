// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import (
	"sync"
	"time"
)

const cacheMaxAge = 5 * time.Minute

func parseLiteralIP(addr string) string {
	var ip IP
	var zone string
	ip = parseIPv4(addr)
	if ip == nil {
		ip, zone = parseIPv6(addr, true)
	}
	if ip == nil {
		return ""
	}
	if zone == "" {
		return ip.String()
	}
	return ip.String() + "%" + zone
}

// hosts contains known host entries.
var hosts struct {
	sync.Mutex

	// Key for the list of literal IP addresses must be a host
	// name. It would be part of DNS labels, a FQDN or an absolute
	// FQDN.
	// For now the key is converted to lower case for convenience.
	byName map[string][]string

	// Key for the list of host names must be a literal IP address
	// including IPv6 address with zone identifier.
	// We don't support old-classful IP address notation.
	byAddr map[string][]string

	expire time.Time
	path   string
}

func readHosts() {
	now := time.Now()
	hp := testHookHostsPath
	if len(hosts.byName) == 0 || now.After(hosts.expire) || hosts.path != hp {
		hs := make(map[string][]string)
		is := make(map[string][]string)
		var file *file
		if file, _ = open(hp); file == nil {
			return
		}
		for line, ok := file.readLine(); ok; line, ok = file.readLine() {
			if i := byteIndex(line, '#'); i >= 0 {
				// Discard comments.
				line = line[0:i]
			}
			f := getFields(line)
			if len(f) < 2 {
				continue
			}
			addr := parseLiteralIP(f[0])
			if addr == "" {
				continue
			}
			for i := 1; i < len(f); i++ {
				name := absDomainName([]byte(f[i]))
				h := []byte(f[i])
				lowerASCIIBytes(h)
				key := absDomainName(h)
				hs[key] = append(hs[key], addr)
				is[addr] = append(is[addr], name)
			}
		}
		// Update the data cache.
		hosts.expire = now.Add(cacheMaxAge)
		hosts.path = hp
		hosts.byName = hs
		hosts.byAddr = is
		file.close()
	}
}

// lookupStaticHost looks up the addresses for the given host from /etc/hosts.
func lookupStaticHost(host string) []string {
	hosts.Lock()
	defer hosts.Unlock()
	readHosts()
	if len(hosts.byName) != 0 {
		// TODO(jbd,bradfitz): avoid this alloc if host is already all lowercase?
		// or linear scan the byName map if it's small enough?
		lowerHost := []byte(host)
		lowerASCIIBytes(lowerHost)
		if ips, ok := hosts.byName[absDomainName(lowerHost)]; ok {
			return ips
		}
	}
	return nil
}

// lookupStaticAddr looks up the hosts for the given address from /etc/hosts.
func lookupStaticAddr(addr string) []string {
	hosts.Lock()
	defer hosts.Unlock()
	readHosts()
	addr = parseLiteralIP(addr)
	if addr == "" {
		return nil
	}
	if len(hosts.byAddr) != 0 {
		if hosts, ok := hosts.byAddr[addr]; ok {
			return hosts
		}
	}
	return nil
}