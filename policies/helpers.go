package policies

import (
	"sync/atomic"
	"time"

	"github.com/puzpuzpuz/xsync/v3"
)

func startRateLimitSystem[K comparable](
	tokensPerInterval int,
	interval time.Duration,
	maxTokens int,
) func(key K) (ratelimited bool) {
	negativeBuckets := xsync.NewMapOf[K, *atomic.Int32]()
	maxTokensInt32 := int32(maxTokens)

	go func() {
		for {
			time.Sleep(interval)
			negativeBuckets.Range(func(key K, bucket *atomic.Int32) bool {
				newv := bucket.Add(int32(-tokensPerInterval))
				if newv <= 0 {
					negativeBuckets.Delete(key)
				}
				return true
			})
		}
	}()

	return func(key K) bool {
		nb, _ := negativeBuckets.LoadOrStore(key, &atomic.Int32{})

		if nb.Load() < maxTokensInt32 {
			nb.Add(1)
			// rate limit not reached yet
			return false
		}

		// rate limit reached
		return true
	}
}
