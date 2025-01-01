package khatru

import (
	"net"
	"net/http"
	"strings"

	"github.com/nbd-wtf/go-nostr"
)

func isOlder(previous, next *nostr.Event) bool {
	return previous.CreatedAt < next.CreatedAt ||
		(previous.CreatedAt == next.CreatedAt && previous.ID > next.ID)
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
