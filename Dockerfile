# First stage: build the app
FROM golang:1 as build

ENV GO111MODULE on
WORKDIR /go/src/app
COPY ./ .

RUN go get -v
RUN go build -v

# Build the actual image
FROM scratch
COPY --from=build /go/src/app/mercure /
COPY public ./public/
CMD ["./mercure"]
EXPOSE 80 443
