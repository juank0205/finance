# syntax=docker/dockerfile:1
# One Dockerfile for every Go service. Pick the service with CMD and the runtime with --target:
#   docker buildx build --platform linux/arm64 --target edge   --build-arg CMD=ingest-api -t finance/ingest-api .
#   docker buildx build --platform linux/arm64 --target lambda --build-arg CMD=cloud-api  -t finance/cloud-api  .
# TODO: pin base images by digest once builds are stable (Dependabot keeps them updated).

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG CMD
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${CMD}

FROM public.ecr.aws/awsguru/aws-lambda-adapter:0.9.1 AS adapter

# Lambda: plain HTTP server + Lambda Web Adapter extension, so the same binary runs locally and in AWS.
FROM gcr.io/distroless/static-debian12:nonroot AS lambda
COPY --from=adapter /lambda-adapter /opt/extensions/lambda-adapter
COPY --from=build /out/app /app
ENV AWS_LWA_READINESS_CHECK_PATH=/healthz
ENTRYPOINT ["/app"]

# Edge (Raspberry Pi): static binary, no shell, non-root.
FROM gcr.io/distroless/static-debian12:nonroot AS edge
COPY --from=build /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
