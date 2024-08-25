---
outline: deep
---

# NIP-42 `AUTH`

`khatru` supports [NIP-42](https://nips.nostr.com/42) out of the box. The functionality is exposed in the following ways.

## Sending arbitrary `AUTH` challenges

At any time you can send an `AUTH` message to a client that is making a request.

It makes sense to give the user the option to authenticate right after they establish a connection, for example, when you have a relay that works differently depending on whether the user is authenticated or not.

```go
relay := khatru.NewRelay()

relay.OnConnect = append(relay.OnConnect, func(ctx context.Context) {
	khatru.RequestAuth(ctx)
})
```

This will send a NIP-42 `AUTH` challenge message to the client so it will have the option to authenticate itself whenever it wants to.

## Signaling to the client that a specific query requires an authenticated user

If on `RejectFilter` or `RejectEvent` you prefix the message with `auth-required: `, that will automatically send an `AUTH` message before a `CLOSED` or `OK` with that prefix, such that the client will immediately be able to know it must authenticate to proceed and will already have the challenge required for that, so they can immediately replay the request.

```go
relay.RejectFilter = append(relay.RejectFilter, func(ctx context.Context, filter nostr.Filter) (bool, string) {
	return true, "auth-required: this query requires you to be authenticated"
})
relay.RejectEvent = append(relay.RejectEvent, func(ctx context.Context, event *nostr.Event) (bool, string) {
	return true, "auth-required: publishing this event requires authentication"
})
```

## Reading the auth status of a client

After a client is authenticated and opens a new subscription with `REQ` or sends a new event with `EVENT`, you'll be able to read the public key they're authenticated with.

```go
relay.RejectFilter = append(relay.RejectFilter, func(ctx context.Context, filter nostr.Filter) (bool, string) {
	authenticatedUser := khatru.GetAuthed(ctx)
})
```

## Telling an authenticated user they're still not allowed to do something

If the user is authenticated but still not allowed (because some specific filters or events are only accessible to some specific users) you can reply on `RejectFilter` or `RejectEvent` with a message prefixed with `"restricted: "` to make that clear to clients.

```go
relay.RejectFilter = append(relay.RejectFilter, func(ctx context.Context, filter nostr.Filter) (bool, string) {
	authenticatedUser := khatru.GetAuthed(ctx)

	if slices.Contain(authorizedUsers, authenticatedUser) {
		return false
	} else {
		return true, "restricted: you're not a member of the privileged group that can read that stuff"
	}
})
```
