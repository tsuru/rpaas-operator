FROM golang:1.12-alpine AS builder

COPY ./ /go/src/github.com/tsuru/nginx-operator/nginx-sidecar
WORKDIR /go/src/github.com/tsuru/nginx-operator/nginx-sidecar
RUN go build /go/src/github.com/tsuru/nginx-operator/nginx-sidecar/main.go

FROM alpine:3.9
COPY --from=builder /go/src/github.com/tsuru/nginx-operator/nginx-sidecar/main /bin/nginx-sidecar
EXPOSE 59999
CMD ["/bin/nginx-sidecar"]
