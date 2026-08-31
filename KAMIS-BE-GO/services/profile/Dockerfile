# syntax=docker/dockerfile:1
# Build from the repo root: docker build -f services/profile/Dockerfile \
#   --build-arg SERVICE=profile -t profile .
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE=profile
RUN CGO_ENABLED=0 go build -trimpath -o /out/app ./services/${SERVICE}/cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
EXPOSE 8080
ENTRYPOINT ["/app"]
