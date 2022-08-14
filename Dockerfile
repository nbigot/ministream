FROM golang:1.18 AS build
WORKDIR /go/src
COPY account ./account
COPY auth ./auth
COPY config ./config
COPY constants ./constants
COPY cron ./cron
COPY data ./data
COPY docs ./docs
COPY log ./log
COPY rbac ./rbac
COPY stream ./stream
COPY web ./web
COPY main.go .
COPY go.mod .
COPY go.sum .

ENV CGO_ENABLED=0
RUN go get -d -v ./...
RUN go build -a -installsuffix cgo -o ministream .

FROM alpine AS runtime
RUN apk --no-cache add curl
WORKDIR /app
COPY --from=build /go/src/ministream ./
COPY docker-template/config /app/config
COPY docker-template/certs /app/certs
COPY docker-template/data /app/data
EXPOSE 8080/tcp
CMD ["/app/ministream", "-config", "/app/config/config.yaml"]
