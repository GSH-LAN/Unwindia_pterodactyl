FROM golang:1.21.7-alpine AS build-env
ADD . /app
WORKDIR /app
ARG TARGETOS
ARG TARGETARCH
RUN go mod download
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o app ./cmd/unwindia_pterodactyl

# Runtime image
FROM redhat/ubi9-minimal:9.3

RUN rpm -ivh https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm
RUN microdnf -y update && microdnf -y install ca-certificates inotify-tools && microdnf reinstall -y tzdata

COPY --from=build-env /app/app /

ENV TZ=Europe/Berlin

EXPOSE 8080
CMD ["./app"]
