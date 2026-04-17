package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"
)

func TestTryerMax(t *testing.T) {
	const maxRetries = 3

	var (
		tr      = Tryer{Max: maxRetries}
		testErr = fmt.Errorf("test error")
	)

	for i := 0; i <= maxRetries; i++ {
		t.Run(fmt.Sprintf("succeed_on_%d", i), func(t *testing.T) {
			var n int
			err := tr.Try(context.Background(), func(j int) error {
				n = j
				if j == i {
					return nil // success
				}
				return testErr
			})

			if i == maxRetries {
				if n != maxRetries-1 {
					t.Errorf("got n==%d, want %d", n, maxRetries-1)
				}

				if err == nil {
					t.Errorf("got no error, want MaxTriesError")
					return
				}
				// Check that the error is both a MaxTriesError and testErr.

				var maxErr MaxTriesError
				if !errors.As(err, &maxErr) {
					t.Errorf("got %T, want MaxTriesError", err)
				}

				if !errors.Is(err, testErr) {
					t.Errorf("got %v, want %v", err, testErr)
				}

				return
			}

			if n != i {
				t.Errorf("got n==%d, want %d", n, i)
			}
		})
	}
}

func TestTryerUnretryable(t *testing.T) {
	var (
		retriableErr   = fmt.Errorf("retriable error")
		unretriableErr = fmt.Errorf("unretryable error")
	)

	tr := Tryer{
		Max: 3,
		IsRetryable: func(e error) bool {
			return errors.Is(e, retriableErr)
		},
	}

	var n int
	err := tr.Try(context.Background(), func(i int) error {
		n = i
		if i == 0 {
			return retriableErr
		}
		if i == 1 {
			return unretriableErr
		}
		return nil
	})

	if n != 1 {
		t.Errorf("got n==%d, want 1", n)
	}

	if err == nil {
		t.Fatal("got no error, want UnretryableError")
	}

	var unretryable UnretryableError
	if !errors.As(err, &unretryable) {
		t.Errorf("got %T, want UnretryableError", err)
	}
	if !errors.Is(err, unretriableErr) {
		t.Errorf("got %v, want %v", err, unretriableErr)
	}
}

func TestOnRetryableError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		type callInfo struct {
			err   error
			n     int
			delay time.Duration
		}

		var calls []callInfo

		tr := Tryer{
			Max:   3,
			Delay: time.Millisecond,
			OnRetryableError: func(err error, n int, delay time.Duration) {
				calls = append(calls, callInfo{err: err, n: n, delay: delay})
			},
		}

		testErr := fmt.Errorf("test error")

		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			err = tr.Try(context.Background(), func(i int) error {
				if i < 2 {
					return testErr
				}
				return nil
			})
		}()

		// Let the goroutine run through both retries (two sleeps of 1ms each).
		synctest.Wait()
		time.Sleep(time.Millisecond)
		synctest.Wait()
		time.Sleep(time.Millisecond)
		synctest.Wait()

		<-done

		if err != nil {
			t.Fatalf("got error %s, want nil", err)
		}
		if len(calls) != 2 {
			t.Fatalf("got %d calls to OnRetryableError, want 2", len(calls))
		}
		for i, call := range calls {
			if !errors.Is(call.err, testErr) {
				t.Errorf("call %d: got error %v, want %v", i, call.err, testErr)
			}
			if call.n != i+1 {
				t.Errorf("call %d: got n==%d, want %d", i, call.n, i+1)
			}
			if call.delay <= 0 {
				t.Errorf("call %d: got non-positive delay %v", i, call.delay)
			}
		}
	})
}

func TestCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		tr := Tryer{
			Max:   -1,
			Delay: time.Second,
		}

		var (
			n    int
			err  error
			done = make(chan struct{})
		)

		go func() {
			defer close(done)
			err = tr.Try(ctx, func(i int) error {
				n = i
				return fmt.Errorf("test error")
			})
		}()

		// After the first call fails, Try sleeps for Delay before retrying.
		// Wait until it is durably blocked on time.After.
		synctest.Wait()
		// Advance past the first delay → triggers second attempt.
		time.Sleep(time.Second)

		synctest.Wait()
		// Advance past the second delay → triggers third attempt.
		time.Sleep(time.Second)

		synctest.Wait()
		// Now cancel the context while Try is sleeping before a fourth attempt.
		cancel()

		// Try should detect ctx.Done() and return without making another call.
		<-done

		if n != 2 {
			t.Errorf("got n==%d, want 2", n)
		}
		if err == nil {
			t.Fatal("got no error, want ContextError")
		}
		var ctxErr ContextError
		if !errors.As(err, &ctxErr) {
			t.Errorf("got %T, want ContextError", err)
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("got %v, want %v", err, context.Canceled)
		}
	})
}

func TestCalcDelay(t *testing.T) {
	floats := []float64{0.1, 0.9, 0.2, 0.8}

	tr := Tryer{
		Delay:    100 * time.Millisecond,
		MaxDelay: 300 * time.Millisecond,
		Jitter:   50 * time.Millisecond,
		Scale:    0.5,
		Rand: func() float64 {
			result := floats[0]
			floats = floats[1:]
			return result
		},
	}

	want := []time.Duration{
		(100 - 40) * time.Millisecond, // scale = 1, jitter = 50ms * (0.1 * 2 - 1)
		(150 + 40) * time.Millisecond, // scale = 1.5, jitter = 50ms * (0.9 * 2 - 1)
		(225 - 30) * time.Millisecond, // scale = 2.25, jitter = 50ms * (0.2 * 2 - 1)
		330 * time.Millisecond,        // capped at MaxDelay, jitter = 50ms * (0.8 * 2 - 1)
	}

	for i := 0; i < 4; i++ {
		if got := tr.calcDelay(i + 1); got != want[i] {
			t.Errorf("calcDelay(%d) = %v, want %v", i, got, want[i])
		}
	}
}
