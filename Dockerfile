# golang:1.26-alpine
FROM golang@sha256:91eda9776261207ea25fd06b5b7fed8d397dd2c0a283e77f2ab6e91bfa71079d AS builder

WORKDIR /src

COPY . .

RUN mkdir -p /out && \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/renovate-alpine-datasource .

# gcr.io/distroless/static-debian12:nonroot
FROM gcr.io/distroless/static-debian12@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639

COPY --from=builder /out/renovate-alpine-datasource /renovate-alpine-datasource

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/renovate-alpine-datasource"]
