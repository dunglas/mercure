# Cookbooks

## Handling More Simultaneous Connections

If you have errors such as `accept: too many open files.` in your logs, you may need to increase the maximum number of file descriptors allowed by the operating system. To do so, use the `ulimit -n` command.

Example:

    ulimit -n 100000

You may also be interested in spreading the load across several servers using [the HA version](cluster.md).

To reproduce the problem, we provide [a load test](load-test.md) that you can use to stress your infrastructure.


## Using Mercure Behind a Reverse Proxy

Mercure hubs run perfectly well behind reverse proxies.
Here are some configuration for popular proxies:

* [NGINX](nginx.md)
* [Traefik](traefik.md)
