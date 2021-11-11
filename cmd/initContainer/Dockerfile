FROM gcr.io/distroless/static

LABEL maintainer="Kyverno"

ADD ./kyvernopre /kyvernopre

USER 10001

ENTRYPOINT ["./kyvernopre"]
