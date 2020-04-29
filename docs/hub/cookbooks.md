# Cookbooks

## Handling More Simultaneous Connections

If you have errors such as `accept: too many open files.` in your logs, you may need to increase the maximum number of file descriptors allowed by the operating system. To do so, use the `ulimit -n` command.

Example:

    ulimit -n 100000

You may also be interested in spreading the load across several servers using [the HA version](cluster.md).

To reproduce the problem, we provide [a load test](load-test.md) that you can use to stress your infrastructure.

## Monitoring the Hub Using Supervisor

Use the following file as a template to run the Mercure hub with [Supervisor](http://supervisord.org):

```ini
[program:mercure]
command=/path/to/mercure
process_name=%(program_name)s_%(process_num)s
numprocs=1
environment=JWT_KEY="!ChangeMe!"
directory=/tmp
autostart=true
autorestart=true
startsecs=5
startretries=10
user=www-data
redirect_stderr=false
stdout_capture_maxbytes=1MB
stderr_capture_maxbytes=1MB
stdout_logfile=/path/to/logs/mercure.out.log
stderr_logfile=/path/to/logs/mercure.error.log
```

Save this file to `/etc/supervisor/conf.d/mercure.conf`.
Run `supervisorctl reread` and `supervisorctl update` to activate and start the Mercure hub.

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
