# First stage: build the app
FROM golang:1 as build

ENV GO111MODULE on
WORKDIR /go/src/app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

COPY ./ .

RUN go get -v
RUN CGO_ENABLED=0 GOOS=linux go build -v -a -ldflags '-extldflags "-static"' .
RUN chmod +x ./mercure

# Build the actual image
FROM scratch
COPY --from=build /go/src/app/mercure .
COPY public ./public/
CMD ["./mercure"]
EXPOSE 80 443
