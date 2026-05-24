package or

import (
    "sync"
    "time"
)

// Or merges multiple done channels into a single channel that closes when any input channel receives a value.
// If no channels are provided, it returns nil. If a single channel is provided, it returns that channel directly.
func Or(channels ...<-chan interface{}) <-chan interface{} {
    if len(channels) == 0 {
        return nil
    }
    if len(channels) == 1 {
        return channels[0]
    }
    out := make(chan interface{})
    var once sync.Once
    for _, ch := range channels {
        go func(c <-chan interface{}) {
            // Wait for a signal (value or close)
            <-c
            once.Do(func() {
                // send a dummy value and close the channel to unblock receivers
                out <- struct{}{}
                close(out)
            })
        }(ch)
    }
    return out
}

// sig is a helper used in tests and examples.
func sig(after time.Duration) <-chan interface{} {
    c := make(chan interface{})
    go func() {
        defer close(c)
        time.Sleep(after)
    }()
    return c
}
