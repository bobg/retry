package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
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

func TestCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	timeCh := make(chan time.Time, 5)
	defer close(timeCh)

	tr := Tryer{
		Max: -1,
		After: func(time.Duration) <-chan time.Time {
			return timeCh
		},
	}

	var (
		n    int
		err  error
		iter = make(chan int, 5) // receives a value on each invocation of the callback
		done = make(chan struct{})
	)

	defer close(iter)

	go func() {
		err = tr.Try(ctx, func(i int) error {
			defer func() { iter <- i }()
			n = i
			return fmt.Errorf("test error")
		})
		close(done)
	}()

	<-iter
	timeCh <- time.Now()

	<-iter
	timeCh <- time.Now()

	<-iter
	cancel()

	timeCh <- time.Now() // this should not trigger another iteration

	select {
	case <-iter:
		t.Fatal("got another iteration after cancel")

	case <-done:
	}

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
}

func TestCalcDelay(t *testing.T) {
	floats := []float64{0.1, 0.9, 0.2}

	tr := Tryer{
		Delay:  100 * time.Millisecond,
		Jitter: 50 * time.Millisecond,
		Scale:  0.5,
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
	}

	for i := 0; i < 3; i++ {
		if got := tr.calcDelay(i + 1); got != want[i] {
			t.Errorf("calcDelay(%d) = %v, want %v", i, got, want[i])
		}
	}
}
