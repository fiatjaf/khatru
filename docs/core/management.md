---
outline: deep
---

# Management API

[NIP-86](https://nips.nostr.com/86) specifies a set of RPC methods for managing the boring aspects of relays, such as whitelisting or banning users, banning individual events, banning IPs and so on.

All [`khatru.Relay`](https://pkg.go.dev/github.com/fiatjaf/khatru#Relay) instances expose a field `ManagementAPI` with a [`RelayManagementAPI`](https://pkg.go.dev/github.com/fiatjaf/khatru#RelayManagementAPI) instance inside, which can be used for creating handlers for each of the RPC methods.

There is also a generic `RejectAPICall` which is a slice of functions that will be called before any RPC method, if they exist and, if any of them returns true, the request will be rejected.

The most basic implementation of a `RejectAPICall` handler would be one that checks the public key of the caller with a hardcoded public key of the relay owner:

```go
var owner = "<my-own-pubkey>"
var allowedPubkeys = make([]string, 0, 10)

func main () {
	relay := khatru.NewRelay()

	relay.ManagementAPI.RejectAPICall = append(relay.ManagementAPI.RejectAPICall,
		func(ctx context.Context, mp nip86.MethodParams) (reject bool, msg string) {
			user := khatru.GetAuthed(ctx)
			if user != owner {
				return true, "go away, intruder"
			}
			return false, ""
		}
	)

	relay.ManagementAPI.AllowPubKey = func(ctx context.Context, pubkey string, reason string) error {
		allowedPubkeys = append(allowedPubkeys, pubkey)
		return nil
	}
	relay.ManagementAPI.BanPubKey = func(ctx context.Context, pubkey string, reason string) error {
		idx := slices.Index(allowedPubkeys, pubkey)
		if idx == -1 {
			return fmt.Errorf("pubkey already not allowed")
		}
		allowedPubkeys = slices.Delete(allowedPubkeys, idx, idx+1)
	}
}
```

You can also not provide any `RejectAPICall` handler and do the approval specifically on each RPC handler.

In the following example any current member can include any other pubkey, and anyone who was added before is able to remove any pubkey that was added afterwards (not a very good idea, but serves as an example).

```go
var allowedPubkeys = []string{"<my-own-pubkey>"}

func main () {
	relay := khatru.NewRelay()

	relay.ManagementAPI.AllowPubKey = func(ctx context.Context, pubkey string, reason string) error {
		caller := khatru.GetAuthed(ctx)

		if slices.Contains(allowedPubkeys, caller) {
			allowedPubkeys = append(allowedPubkeys, pubkey)
			return nil
		}

		return fmt.Errorf("you're not authorized")
	}
	relay.ManagementAPI.BanPubKey = func(ctx context.Context, pubkey string, reason string) error {
		caller := khatru.GetAuthed(ctx)

		callerIdx := slices.Index(allowedPubkeys, caller)
		if callerIdx == -1 {
			return fmt.Errorf("you're not even allowed here")
		}

		targetIdx := slices.Index(allowedPubkeys, pubkey)
		if targetIdx < callerIdx {
			// target is a bigger OG than the caller, so it has bigger influence and can't be removed
			return fmt.Errorf("you're less powerful than the pubkey you're trying to remove")
		}

		// allow deletion since the target came after the caller
		allowedPubkeys = slices.Delete(allowedPubkeys, targetIdx, targetIdx+1)
		return nil
	}
}
```
