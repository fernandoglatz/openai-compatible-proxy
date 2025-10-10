FROM golang:1.25-alpine as go-alpine
RUN apk --no-cache add tzdata
COPY . /go/src
WORKDIR /go/src
RUN go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o main .


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
