# Retry

[![Go Reference](https://pkg.go.dev/badge/github.com/bobg/retry.svg)](https://pkg.go.dev/github.com/bobg/retry)
[![Go Report Card](https://goreportcard.com/badge/github.com/bobg/retry)](https://goreportcard.com/report/github.com/bobg/retry)
[![Tests](https://github.com/bobg/retry/actions/workflows/go.yml/badge.svg)](https://github.com/bobg/retry/actions/workflows/go.yml)
[![Coverage Status](https://coveralls.io/repos/github/bobg/retry/badge.svg?branch=main)](https://coveralls.io/github/bobg/retry?branch=main)

This is retry,
a Go library for retrying function calls that may fail.

It lets you limit the number of retries and the time spent retrying.
You can configure the interval between tries and do exponential backoff.
You can add random jitter to the time interval.
And you can supply a function for discriminating between retryable and non-retryable errors.

## Usage

```go
tr := retry.Tryer{
  Max:   5,
  Delay: 100 * time.Millisecond,
  Scale: 0.25,
}
err := tr.Try(ctx, myFunc)
```

## Seriously, another retry library for Go?

There are already some excellent retry libraries for Go.
But I did not find one to be as complete and as ergonomic as this simple API.
For details, please see [the Godoc](https://pkg.go.dev/github.com/bobg/retry).
