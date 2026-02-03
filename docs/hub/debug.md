# Debug the Mercure.rocks Hub

The hub is shipped with [`pprof`](https://blog.golang.org/pprof),
a profiler that helps you find bottlenecks, memory leaks, and blocked goroutines,
among other things.

To enable the profiler, add the `debug` global directive to your `Caddyfile`:

```Caddyfile
{
  debug
}

# ...
```

If you use the default `Caddyfile`, you can also set the `GLOBAL_OPTIONS` environment variable to `debug`.

The route exposing profiling data is now available at `http://localhost:2019/debug/pprof/`.
You can use [the `pprof` tool](https://golang.org/pkg/net/http/pprof/) to visualize this data.

## Examples

View the heap profile:

```console
go tool pprof -http=:8080 http://localhost:2019/debug/pprof/heap
```

View past memory allocations:

```console
go tool pprof -http=:8080 http://localhost:2019/debug/pprof/allocs
```

See `http://localhost:2019/debug/pprof/` for the full list of available data.
