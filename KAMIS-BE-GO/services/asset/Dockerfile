# syntax=docker/dockerfile:1
# Build context is the repo root (KAMIS-BE-GO), so the whole module is available.
# Pick the service to build with --build-arg SERVICE=<name>.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE=asset
RUN CGO_ENABLED=0 go build -trimpath -o /out/app ./services/${SERVICE}/cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
EXPOSE 8081
ENTRYPOINT ["/app"]
