FROM scratch
ADD kyverno /kyverno
ENTRYPOINT ["/kyverno"]
