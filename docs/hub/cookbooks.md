# Cookbooks

## Handling More Simultaneous Connections

If you have errors such as `accept: too many open files.` in your logs, you may need to increase the maximum number of file descriptors allowed by the operating system. To do so, use the `ulimit -n` command.

Example:

    ulimit -n 100000

You may also be interested in spreading the load across several servers using [the HA version](cluster.md).

To reproduce the problem, we provide [a load test](load-test.md) that you can use to stress your infrastructure.

## Using NGINX as an HTTP/2 Reverse Proxy in Front of the Hub

[NGINX](https://www.nginx.com) is supported out of the box. Use the following proxy configuration:

```nginx
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;

    ssl_certificate /path/to/ssl/cert.crt;
    ssl_certificate_key /path/to/ssl/cert.key;

    location / {
        proxy_pass http://url-of-your-mercure-hub;
        proxy_read_timeout 24h;
        proxy_http_version 1.1;
        proxy_set_header Connection "";

        ## Be sure to set USE_FORWARDED_HEADERS=1 to allow the hub to use those headers ##
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```
