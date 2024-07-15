---
outline: deep
---

# Generating custom live events

Suppose you want to generate a new event every time a goal is scored on some soccer game and send that to all clients subscribed to a given game according to a tag `t`.

We'll assume you'll be polling some HTTP API that gives you the game's current score, and that in your `main` function you'll start the function that does the polling:

```go
func main () {
	// other stuff here
	relay := khatru.NewRelay()

	go startPollingGame(relay)
	// other stuff here
}

type GameStatus struct {
	TeamA int `json:"team_a"`
	TeamB int `json:"team_b"`
}

func startPollingGame(relay *khatru.Relay) {
	current := GameStatus{0, 0}

	for {
		newStatus, err := fetchGameStatus()
		if err != nil {
				continue
		}

		if newStatus.TeamA > current.TeamA {
				// team A has scored a goal, here we generate an event
				evt := nostr.Event{
					CreatedAt: nostr.Now(),
					Kind: 1,
					Content: "team A has scored!",
					Tags: nostr.Tags{{"t", "this-game"}}
				}
				evt.Sign(global.RelayPrivateKey)
				// calling BroadcastEvent will send the event to everybody who has been listening for tag "t=[this-game]"
				// there is no need to do any code to keep track of these clients or who is listening to what, khatru
				// does that already in the background automatically
				relay.BroadcastEvent(evt)

				// just calling BroadcastEvent won't cause this event to be be stored,
				// if for any reason you want to store these events you must call the store functions manually
				for _, store := range relay.StoreEvent {
					store(context.TODO(), evt)
				}
		}
		if newStatus.TeamB > current.TeamB {
				// same here, if team B has scored a goal
				// ...
		}
	}
}

func fetchGameStatus() (GameStatus, error) {
	// implementation of calling some external API goes here
}
```
