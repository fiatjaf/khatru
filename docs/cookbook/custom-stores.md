---
outline: deep
---

# Generating events on the fly from a non-Nostr data-source

Suppose you want to serve events with the weather data for periods in the past. All you have is a big CSV file with the data.

Then you get a query like `{"#g": ["d6nvp"], "since": 1664074800, "until": 1666666800, "kind": 10774}`, imagine for a while that kind `10774` means weather data.

First you do some geohashing calculation to discover that `d6nvp` corresponds to Willemstad, Curaçao, then you query your XML file for the Curaçao weather data for the given period -- from `2022-09-25` to `2022-10-25`, then you return the events corresponding to such query, signed on the fly:

```go
func main () {
	// other stuff here
	relay := khatru.NewRelay()

	relay.QueryEvents = append(relay.QueryEvents,
		handleWeatherQuery,
	)
	// other stuff here
}

func handleWeatherQuery(ctx context.Context, filter nostr.Filter) (ch chan *nostr.Event, err error) {
	if filter.Kind != 10774 {
		// this function only handles kind 10774, if the query is for something else we return
		// a nil channel, which corresponds to no results
		return nil, nil
	}

	file, err := os.Open("weatherdata.xml")
	if err != nil {
		return nil, fmt.Errorf("we have lost our file: %w", err)
	}

	// QueryEvents functions are expected to return a channel
	ch := make(chan *nostr.Event)

	// and they can do their query asynchronously, emitting events to the channel as they come
	go func () {
		defer file.Close()

		// we're going to do this for each tag in the filter
		gTags, _ := filter.Tags["g"]
		for _, gTag := range gTags {
			// translate geohash into city name
			citName, err := geohashToCityName(gTag)
			if err != nil {
				continue
			}

			reader := csv.NewReader(file)
			for {
				record, err := reader.Read()
				if err != nil {
					return
				}

				// ensure we're only getting records for Willemstad
				if cityName != record[0] {
					continue
				}

				date, _ := time.Parse("2006-01-02", record[1])
				ts := nostr.Timestamp(date.Unix())
				if ts > filter.Since && ts < filter.Until {
					// we found a record that matches the filter, so we make
					// an event on the fly and return it
					evt := nostr.Event{
						CreatedAt: ts,
						Kind: 10774,
						Tags: nostr.Tags{
							{"temperature", record[2]},
							{"condition", record[3]},
						}
					}
					evt.Sign(global.RelayPrivateKey)
					ch <- evt
				}
			}
		}
	}()

	return ch, nil
}
```

Beware, the code above is inefficient and the entire approach is not very smart, it's meant just as an example.
