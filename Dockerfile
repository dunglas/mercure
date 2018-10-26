FROM scratch
COPY mercure /
COPY public ./public/
CMD ["./mercure"]
EXPOSE 80 443
