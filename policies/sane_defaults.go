package policies

import (
	"time"

	"github.com/fiatjaf/khatru"
)

func ApplySaneDefaults(relay *khatru.Relay) {
	relay.RejectEvent = append(relay.RejectEvent,
		RejectEventsWithBase64Media,
		EventIPRateLimiter(2, time.Minute*3, 5),
	)

	relay.RejectFilter = append(relay.RejectFilter,
		NoEmptyFilters,
		NoComplexFilters,
		FilterIPRateLimiter(20, time.Minute, 100),
	)

	relay.RejectConnection = append(relay.RejectConnection,
		ConnectionRateLimiter(1, time.Minute*5, 3),
	)
}
