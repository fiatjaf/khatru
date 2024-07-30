package khatru

import (
	"github.com/nbd-wtf/go-nostr"
)

type Router struct{ *Relay }

type Route struct {
	eventMatcher  func(*nostr.Event) bool
	filterMatcher func(nostr.Filter) bool
	relay         *Relay
}

type routeBuilder struct {
	router        *Router
	eventMatcher  func(*nostr.Event) bool
	filterMatcher func(nostr.Filter) bool
}

func NewRouter() *Router {
	rr := &Router{Relay: NewRelay()}
	rr.routes = make([]Route, 0, 3)
	rr.getSubRelayFromFilter = func(f nostr.Filter) *Relay {
		for _, route := range rr.routes {
			if route.filterMatcher(f) {
				return route.relay
			}
		}
		return rr.Relay
	}
	rr.getSubRelayFromEvent = func(e *nostr.Event) *Relay {
		for _, route := range rr.routes {
			if route.eventMatcher(e) {
				return route.relay
			}
		}
		return rr.Relay
	}
	return rr
}

func (rr *Router) Route() routeBuilder {
	return routeBuilder{
		router:        rr,
		filterMatcher: func(f nostr.Filter) bool { return false },
		eventMatcher:  func(e *nostr.Event) bool { return false },
	}
}

func (rb routeBuilder) Req(fn func(nostr.Filter) bool) routeBuilder {
	rb.filterMatcher = fn
	return rb
}

func (rb routeBuilder) Event(fn func(*nostr.Event) bool) routeBuilder {
	rb.eventMatcher = fn
	return rb
}

func (rb routeBuilder) Relay(relay *Relay) {
	rb.router.routes = append(rb.router.routes, Route{
		filterMatcher: rb.filterMatcher,
		eventMatcher:  rb.eventMatcher,
		relay:         relay,
	})
}
