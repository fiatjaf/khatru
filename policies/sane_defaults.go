package policies

import "github.com/fiatjaf/khatru"

func ApplySaneDefaults(relay *khatru.Relay) {
	relay.RejectEvent = append(relay.RejectEvent,
		RejectEventsWithBase64Media,
	)

	relay.RejectFilter = append(relay.RejectFilter,
		NoEmptyFilters,
		NoComplexFilters,
	)
}
