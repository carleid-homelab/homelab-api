# syntax=docker/dockerfile:1

# ---- build ----
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /homelab-api .

# ---- run ----
FROM gcr.io/distroless/static:nonroot
COPY --from=build /homelab-api /homelab-api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/homelab-api"]
