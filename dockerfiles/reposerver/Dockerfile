FROM golang:1.15.4-alpine3.12

RUN mkdir -p /go/src/github.com/nikogura/dbt

COPY . /go/src/github.com/nikogura/dbt

WORKDIR /go/src/github.com/nikogura/dbt

RUN go build github.com/nikogura/dbt/cmd/reposerver

RUN mv reposerver /usr/local/bin

RUN cp dockerfiles/reposerver/entrypoint.sh /

WORKDIR /

RUN mkdir /var/dbt
RUN mkdir /etc/dbt

ENV ADDRESS 0.0.0.0
ENV PORT 9999
ENV SERVER_ROOT /var/dbt
ENV CONFIG_FILE /etc/dbt/reposerver.json

ENTRYPOINT ["/entrypoint.sh"]
