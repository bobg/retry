package retry_test

import (
	"context"
	"fmt"
	"time"

	"github.com/bobg/retry"
)

func ExampleTryer() {
	// With the following config,
	// tr.Try will try to execute the function up to 5 times.
	// It will wait 100ms after the first attempt, plus or minus up to 50ms of jitter;
	// 150 (100 × 1.5) after the second, plus or minus up to 50ms;
	// 225 (100 × 1.5 × 1.5) after the third, plus or minus up to 50ms;
	// etc.
	tr := retry.Tryer{
		Max:    5,
		Delay:  100 * time.Millisecond,
		Jitter: 50 * time.Millisecond,
		Scale:  0.5,
	}

	// This context makes sure tr.Try spends no more than about 1 second doing retries.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Retry a simple function that fails on its first two tries and succeeds on its third.
	err := tr.Try(ctx, func(n int) error {
		if n < 2 {
			return fmt.Errorf("failed on try #%d", n)
		}
		fmt.Printf("Succeeded on try #%d\n", n)
		return nil
	})

	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}

	// Output:
	// Succeeded on try #2
}
