FROM golang:alpine3.8 AS builder
COPY . /go/src/qiniu-ava/snapshot-operator/
RUN CGO_ENABLED=0 go build -o /go/bin/snapshot-worker qiniu-ava/snapshot-operator/cmd/snapshot-worker

FROM alpine:3.8
COPY --from=builder /go/bin/snapshot-worker /
ENTRYPOINT [ "/snapshot-worker" ]
