FROM golang:1-alpine AS builder
WORKDIR /go/src/github.com/dunglas/mercure/
RUN apk --no-cache add git
COPY main.go .
COPY hub ./hub
RUN go get -d -v ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mercure .

FROM scratch
COPY public .
COPY --from=builder /go/src/github.com/dunglas/mercure/mercure .
CMD ["./mercure"]
EXPOSE 80 443
