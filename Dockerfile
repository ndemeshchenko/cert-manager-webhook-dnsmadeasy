FROM golang:1.17.5-alpine AS build
RUN apk add --no-cache git
WORKDIR /workspace
ENV GO111MODULE=on

COPY scripts scripts
COPY testdata testdata
# RUN ./scripts/fetch-test-binaries.sh
COPY go.mod .
COPY go.sum .
COPY main.go .
COPY main_test.go .

RUN ls -la
RUN go mod download
RUN CGO_ENABLED=0 go build -o webhook -ldflags '-w -extldflags "-static"' .

# ------------------------------
FROM alpine:3.14.2
RUN apk add --no-cache ca-certificates
COPY --from=build /workspace/webhook /usr/local/bin/webhook
ENTRYPOINT ["webhook"]
