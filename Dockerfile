FROM caddy:2-alpine
COPY mercure /usr/bin/caddy
COPY Caddyfile /etc/caddy/Caddyfile
RUN sed -i 's/#transport_url/transport_url/' /etc/caddy/Caddyfile
COPY public public/
