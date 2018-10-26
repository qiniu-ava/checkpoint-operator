FROM golang:alpine3.8 AS builder
COPY . /go/src/github.com/qiniu-ava/snapshot-operator/
RUN CGO_ENABLED=0 go build -o /go/bin/snapshot-operator github.com/qiniu-ava/snapshot-operator/cmd/snapshot-operator

FROM alpine:3.8
COPY --from=builder /go/bin/snapshot-operator /
ENTRYPOINT [ "/snapshot-operator" ]
