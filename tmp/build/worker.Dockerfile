FROM golang:alpine3.8 AS builder
COPY . /go/src/qiniu-ava/checkpoint-operator/
RUN CGO_ENABLED=0 go build -o /go/bin/checkpoint-worker qiniu-ava/checkpoint-operator/cmd/checkpoint-worker

FROM alpine:3.8
COPY --from=builder /go/bin/checkpoint-worker /
ENTRYPOINT [ "/checkpoint-worker" ]
