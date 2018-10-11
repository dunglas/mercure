FROM scratch
COPY mercure /
COPY public .
CMD ["./mercure"]
EXPOSE 80 443
