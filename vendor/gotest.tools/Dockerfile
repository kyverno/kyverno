
ARG     GOLANG_VERSION
FROM    golang:${GOLANG_VERSION:-1.11-alpine} as golang
RUN     apk add -U curl git bash
WORKDIR /go/src/gotest.tools
ENV     CGO_ENABLED=0
ENV     PS1="# "


FROM    golang as tools
ARG     FILEWATCHER=v0.3.2
RUN     go get -d github.com/dnephin/filewatcher && \
        cd /go/src/github.com/dnephin/filewatcher && \
        git checkout -q "$FILEWATCHER" && \
        go build -o /usr/bin/filewatcher .

ARG     DEP_TAG=v0.4.1
RUN     go get -d github.com/golang/dep/cmd/dep && \
        cd /go/src/github.com/golang/dep && \
        git checkout -q "$DEP_TAG" && \
        go build -o /usr/bin/dep ./cmd/dep

ARG     GOTESTSUM=v0.3.2
RUN     go get -d gotest.tools/gotestsum && \
        cd /go/src/gotest.tools/gotestsum && \
        git checkout -q "$GOTESTSUM" && \
        go build -v -o /usr/bin/gotestsum .


FROM    golang as dev
COPY    --from=tools /usr/bin/filewatcher /usr/bin/filewatcher
COPY    --from=tools /usr/bin/dep /usr/bin/dep
COPY    --from=tools /usr/bin/gotestsum /usr/bin/gotestsum


FROM    dev as dev-with-source
COPY    . .


FROM    golang as linter
ARG     GOMETALINTER=v2.0.11
RUN     go get -d github.com/alecthomas/gometalinter && \
        cd /go/src/github.com/alecthomas/gometalinter && \
        git checkout -q "$GOMETALINTER" && \
        go build -v -o /usr/local/bin/gometalinter . && \
        gometalinter --install && \
        rm -rf /go/src/* /go/pkg/*
ENTRYPOINT ["/usr/local/bin/gometalinter"]
CMD     ["--config=.gometalinter.json", "./..."]


FROM    linter as linter-with-source
COPY    . .
