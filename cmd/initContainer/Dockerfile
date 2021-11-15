FROM gcr.io/distroless/static

ARG TARGETPLATFORM

LABEL maintainer="Kyverno"

COPY ./${TARGETPLATFORM}/kyvernopre /kyvernopre

USER 10001

ENTRYPOINT ["./kyvernopre"]
