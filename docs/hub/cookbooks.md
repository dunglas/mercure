# Cookbooks

## Handling More Simultaneous Connections

If you see errors such as `accept: too many open files.` in your logs, you may need to increase the maximum number of file descriptors allowed by your operating system. Use the `ulimit -n` command to do this.

Example:

    ulimit -n 100000

You may also want to distribute the load across several servers using [the Enterprise version](cluster.md).

To reproduce this problem, we provide [a load test](load-test.md) that you can use to stress your infrastructure.

## Using Mercure Behind a Reverse Proxy

Mercure hubs work perfectly well behind reverse proxies.
Here are some configurations for popular proxies:

- [NGINX](nginx.md)
- [Traefik](traefik.md)
