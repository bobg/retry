package retry

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// Tryer runs a function via its Try method
// one or more times until it succeeds,
// or a maximum number of retries is reached,
// or it encounters an unretryable error.
//
// It waits for a specified interval between attempts,
// optionally adding a random amount of jitter to the delay.
// The interval can optionally scale up after each attempt,
// for exponential backoff.
//
// There is no MaxTime field.
// To limit the total time spent retrying,
// set a deadline on the context passed to [Tryer.Try].
type Tryer struct {
	// Max is the maximum number of tries to make.
	// [Tryer.Try] always makes at least one attempt.
	// Leaving this set to 0 is the same as setting it to 1.
	// A negative value means there is no limit on the number of attempts.
	Max int

	// Delay is the initial delay between attempts.
	// Be sure to set this to a non-zero value to avoid an expensive busy loop.
	// The delay can increase after each attempt; see the Scale field below.
	Delay time.Duration

	// Jitter is the maximum amount of random jitter to add to,
	// or subtract from,
	// the delay on each attempt.
	// This value is silently limited to each iteration's delay.
	Jitter time.Duration

	// Scale increases the delay after each attempt, multiplying it by 1+Scale.
	// For example, setting this to 1 will double the delay after each attempt.
	// Leaving this set to 0 means the delay will not scale.
	Scale float64

	// MaxDelay is the maximum delay between attempts.
	// Scale will not cause the delay to exceed this value.
	// (However, random Jitter may still be added.)
	// A value of 0 means there is no maximum delay.
	MaxDelay time.Duration

	// IsRetryable is an optional function that determines whether an error is retryable.
	// If it is nil, all errors are considered retryable.
	IsRetryable func(error) bool

	// After is an optional function returning a channel that sends the current time after the specified duration.
	// If it is nil, [time.After] is used.
	After func(time.Duration) <-chan time.Time

	// Rand is an optional function that returns a random float64 in the range [0, 1).
	// If it is nil, [rand.Float64] is used.
	Rand func() float64
}

// Try runs the provided function one or more times until it succeeds,
// or the provided context is canceled,
// or certain other conditions are met - see [Tryer].
//
// The number of the current attempt is passed to the function as an argument,
// starting at 0 for the first attempt.
//
// If f succeeds (i.e., returns nil), Try returns nil.
// Otherwise it returns one of these error-wrapper types:
// [UnretryableError], [MaxTriesError], or [ContextError].
func (tr Tryer) Try(ctx context.Context, f func(int) error) error {
	n := 0

	for {
		err := f(n)
		if err == nil {
			return nil
		}

		n++
		if tr.Max >= 0 && n >= tr.Max {
			return MaxTriesError{Err: err}
		}

		if tr.IsRetryable != nil && !tr.IsRetryable(err) {
			return UnretryableError{Err: err}
		}

		delay := tr.calcDelay(n)

		select {
		case <-ctx.Done():
			return ContextError{Err: ctx.Err()}
		case <-tr.after(delay):
		}
	}
}

// Computes a delay before try number n.
func (tr Tryer) calcDelay(n int) time.Duration {
	delay := tr.Delay
	if tr.Scale > 0 {
		scale := math.Pow(1+tr.Scale, float64(n-1))
		delay = time.Duration(float64(delay) * scale)
	}

	if tr.MaxDelay > 0 && delay > tr.MaxDelay {
		delay = tr.MaxDelay
	}

	jitter := tr.Jitter
	if jitter > delay {
		jitter = delay
	}

	if jitter > 0 {
		rand := 2*tr.randFloat() - 1 // [-1, 1)
		jitter = time.Duration(float64(jitter) * rand)
		delay += jitter
	}

	return delay
}

func (tr Tryer) randFloat() float64 {
	f := tr.Rand
	if f == nil {
		f = rand.Float64
	}
	return f()
}

func (tr Tryer) after(d time.Duration) <-chan time.Time {
	after := tr.After
	if after == nil {
		after = time.After
	}
	return after(d)
}

// UnretryableError is an error returned by [Tryer.Try]
// wrapping the error returned by the function
// when it is determined to be unretryable.
type UnretryableError struct {
	Err error
}

func (e UnretryableError) Error() string {
	return "unretryable error: " + e.Err.Error()
}
func (e UnretryableError) Unwrap() error {
	return e.Err
}

// MaxTriesError is an error returned by [Tryer.Try]
// wrapping the error returned by the function
// after the maximum number of tries is reached.
type MaxTriesError struct {
	Err error
}

func (e MaxTriesError) Error() string {
	return "reached maximum retries: " + e.Err.Error()
}
func (e MaxTriesError) Unwrap() error {
	return e.Err
}

// ContextError is an error returned by [Tryer.Try]
// wrapping the context error when the context is canceled.
type ContextError struct {
	Err error
}

func (e ContextError) Error() string {
	return "context error: " + e.Err.Error()
}
func (e ContextError) Unwrap() error {
	return e.Err
}
