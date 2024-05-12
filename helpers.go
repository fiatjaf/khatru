package khatru

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
)

func isOlder(previous, next *nostr.Event) bool {
	return previous.CreatedAt < next.CreatedAt ||
		(previous.CreatedAt == next.CreatedAt && previous.ID > next.ID)
}

func getServiceBaseURL(r *http.Request) string {
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if host == "localhost" {
			proto = "http"
		} else if strings.Index(host, ":") != -1 {
			// has a port number
			proto = "http"
		} else if _, err := strconv.Atoi(strings.ReplaceAll(host, ".", "")); err == nil {
			// it's a naked IP
			proto = "http"
		} else {
			proto = "https"
		}
	}
	return proto + "://" + host
}

var privateMasks = func() []net.IPNet {
	privateCIDRs := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7",
	}
	masks := make([]net.IPNet, len(privateCIDRs))
	for i, cidr := range privateCIDRs {
		_, netw, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil
		}
		masks[i] = *netw
	}
	return masks
}()

func isPrivate(ip net.IP) bool {
	for _, mask := range privateMasks {
		if mask.Contains(ip) {
			return true
		}
	}
	return false
}

func GetIPFromRequest(r *http.Request) string {
	if xffh := r.Header.Get("X-Forwarded-For"); xffh != "" {
		for _, v := range strings.Split(xffh, ",") {
			if ip := net.ParseIP(strings.TrimSpace(v)); ip != nil && ip.IsGlobalUnicast() && !isPrivate(ip) {
				return ip.String()
			}
		}
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
