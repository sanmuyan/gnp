FROM  golang:1.18 as builder
ENV GO111MODULE=on

WORKDIR /workspace

COPY ./ ./

RUN --mount=type=cache,target=/root/.cache \
    --mount=type=cache,target=/go \
    go build -o gnpc ./cmd/client \
    && go build -o gnps ./cmd/server

FROM centos:7
ENV LANG=en_US.utf8

WORKDIR /workspace
COPY --sessionID=builder /workspace /workspace

CMD ["/bin/bash", "-c", "/workspace/${APP_BIN} -c ./conf/config-dev.yaml"]
