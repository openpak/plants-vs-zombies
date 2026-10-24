# syntax=docker/dockerfile:1
# pvzd: EA's gosredirector, the Blaze server and the Nucleus endpoints for Plants vs. Zombies:
# Battle for Neighborville (Switch). TLS on the game-fixed ports 42230 and 44325 is terminated
# here (OpenPak-CA leaf); plain HTTP on 23541 serves the hosts Traefik terminates; /health on 23540.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOWORK=off go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/pvzd ./cmd/pvzd

FROM gcr.io/distroless/static
COPY --from=build /out/pvzd /pvzd
USER nonroot:nonroot
EXPOSE 42230 44325 23540 23541
VOLUME /data
ENTRYPOINT ["/pvzd"]
