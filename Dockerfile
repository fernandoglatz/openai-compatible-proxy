# Build arguments for multi-platform builds
ARG BUILDPLATFORM
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS go-alpine
RUN apk --no-cache add tzdata
WORKDIR /go/src

# Copy go.mod and go.sum first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build for the target platform
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o main .


FROM scratch
WORKDIR /app

COPY --from=go-alpine /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=go-alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=go-alpine /go/src/main /app
COPY conf /app/conf
COPY scripts /app/scripts

EXPOSE 8080

ENV TZ=America/Sao_Paulo

CMD ["/app/main"]
