FROM alpine:3.6

RUN adduser -D checkpoint-operator
USER checkpoint-operator

ADD tmp/_output/bin/checkpoint-operator /usr/local/bin/checkpoint-operator
