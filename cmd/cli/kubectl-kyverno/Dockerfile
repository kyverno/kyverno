FROM gcr.io/distroless/static

LABEL maintainer="Kyverno"

ADD ./kyverno /kyverno

USER 10001

ENTRYPOINT ["./kyverno"]
