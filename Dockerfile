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

# Build healthcheck binary
RUN printf 'package main\nimport(\n"net/http"\n"os"\n)\nfunc main(){\nresp,err:=http.Get("http://localhost:8080/health")\nif err!=nil||resp.StatusCode!=200{os.Exit(1)}\n}' > /tmp/healthcheck.go && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o healthcheck /tmp/healthcheck.go

# scratch has no /tmp, and modernc.org/sqlite needs a writable temp dir for some
# operations. Stage an empty one to copy into the final image.
RUN mkdir -p /staging/tmp && chmod 1777 /staging/tmp


FROM scratch
WORKDIR /app

COPY --from=go-alpine /staging/tmp /tmp
COPY --from=go-alpine /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=go-alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=go-alpine /go/src/main /app
COPY --from=go-alpine /go/src/healthcheck /app
COPY conf /app/conf
COPY scripts /app/scripts

EXPOSE 8080

ENV TZ=America/Sao_Paulo

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/healthcheck"]

CMD ["/app/main"]
