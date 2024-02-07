FROM gcr.io/distroless/static:nonroot
ARG TARGETARCH
COPY bin/ocppprom-linux-${TARGETARCH} /bin/ocppprom
ENTRYPOINT ["/bin/ocppprom"]
