# Debug the Mercure.rocks Hub

The hub is shipped with [`pprof`](https://blog.golang.org/pprof),
a profiler allowing to find bottlenecks, memory leaks and blocked goroutines 
among other things.

To enable the profiler, add the `debug` global directive to your `Caddyfile`:

```Caddyfile
{
    debug
}

# ...
```

The route exposing profiling data is now available at `http://localhost:2019/debug/pprof/`.
You can use [the `pprof` tool](https://golang.org/pkg/net/http/pprof/) to visualize it.

## Examples

Tip: type `web` when in interative mode to display a visual representation of the profile. 

Look at the heap profile:

   go tool pprof http://localhost:2019/debug/pprof/heap

Look at the past memory allocations:

    go tool pprof http://localhost:2019/debug/pprof/allocs

See `http://localhost:2019/debug/pprof/` for the list of available data.
