FROM golang:1.19 AS build
WORKDIR /go/src
COPY . .
ENV CGO_ENABLED=0
RUN go mod download
RUN go build -a -o /go/bin/ministream /go/src/cmd/ministream/ministream.go
RUN go build -a -o /go/bin/generatepasswords /go/src/cmd/generatepasswords/generatepasswords.go

FROM alpine AS runtime
RUN apk --no-cache add curl
WORKDIR /app
COPY --from=build /go/bin/* ./
COPY config-templates/docker/config /app/config
COPY config-templates/docker/certs /app/certs
COPY config-templates/docker/data /app/data
EXPOSE 8080/tcp
CMD ["/app/ministream", "-config", "/app/config/config.yaml"]
