FROM gcr.io/distroless/static

ARG TARGETPLATFORM

LABEL maintainer="Kyverno"

COPY ./${TARGETPLATFORM}/kyverno /kyverno

USER 10001

ENTRYPOINT ["./kyverno"]
