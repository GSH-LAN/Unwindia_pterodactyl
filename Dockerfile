FROM golang:1.20.2-alpine AS build-env
ADD . /app
WORKDIR /app
ARG TARGETOS
ARG TARGETARCH
RUN go mod download
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o app ./cmd/unwindia_pterodactyl

# Runtime image
FROM redhat/ubi8-minimal:8.7

RUN rpm -ivh https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
RUN microdnf update && microdnf -y install ca-certificates inotify-tools

COPY --from=build-env /app/app /
EXPOSE 8080
CMD ["./app"]
