[![PkgGoDev](https://pkg.go.dev/badge/github.com/polarsignals/rprof)](https://pkg.go.dev/github.com/polarsignals/rprof)

# rprof

Profiling I/O reads in Go. This in-process profiler for Go profiles the amount, number and size of reads that occur on any `io.Reader`, `io.ReadCloser`, or `io.ReaderAt` implementation.

# Why?

At Polar Signals, we need to understand reads to object storage since every round trip costs money, but more importantly has ~100ms latency. We need to understand how to optimize reads to object storage.

# How?

This package provides a `Reader` implementation that wraps any `io.Reader` implementation and profiles reads. The `Reader` implementation is a `io.Reader` itself, so it can be used anywhere an `io.Reader` is expected.

Every time a read occurs, the `Reader` implementation will record number of bytes read bucket them into their respective power of two size and record the stack that lead to the read. The size of the read is attached as a label to the stack trace, so it can be differentiated later what sizes of reads were performed.

# Usage

An example of how to use this package can be found in the `extern_test.go` file. You can run `go test -c` to compile the tests and then `./rprof.test -test.v` to run the tests. The tests will output a pprof profile that can be analyzed with `go tool pprof -http=:8080 profile.pb.gz`.

A typical usage may look like this:

```go
rprof.Reader(reader) // use this reader where you would normally use reader

if err := rprof.Start(); err != nil {
    // handle error
}

time.Sleep(10 * time.Second) // let it accumulate some data for 10 seconds
prof, err := rprof.Stop()
if err != nil {
    // handle error
}

// prof is a pprof profile that can now be written to disk, or returned on an HTTP endpoint
```

Or if you expose the profile on an HTTP endpoint:

```go
r := rprof.Reader(reader)
// Use reader wherever you would normally use the original reader

// Expose on an HTTP endpoint
http.HandleFunc("/debug/rprof", rprof.Handler())
```
