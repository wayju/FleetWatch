package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// checkTransport applies checkURL and then the cleartext policy. Upstream and
// recap requests carry a bearer token, so plain http is accepted only to a
// loopback host, or when the entry sets allow_insecure for a link the operator
// secures another way (a VPN, a tunnel). That last case is logged as a warning.
func (r *Resolved) checkTransport(field, u string, allowInsecure bool) error {
	host, err := checkURL(field, u)
	if err != nil {
		return err
	}
	p, _ := url.Parse(u)
	if p.Scheme != "http" || isLoopbackHost(host) {
		return nil
	}
	if !allowInsecure {
		return fmt.Errorf("%s %q is plain http to a non-loopback host; use https, or set allow_insecure true if the link is secured another way", field, u)
	}
	r.Warnings = append(r.Warnings, fmt.Sprintf("WARNING: %s %s is plain http (allow_insecure): its bearer token travels in clear", field, host))
	return nil
}

// isLoopbackHost reports whether host is localhost or a loopback address
// (127.0.0.0/8, ::1). Names other than localhost are never trusted to resolve
// to loopback.
func isLoopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
