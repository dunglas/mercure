FROM caddy:2-alpine
COPY mercure /usr/bin/caddy
COPY Caddyfile /etc/caddy/Caddyfile
COPY public public/
