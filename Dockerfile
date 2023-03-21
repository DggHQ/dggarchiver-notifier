FROM golang:bullseye as builder
ARG TARGETARCH
LABEL builder=true multistage_tag="dggarchiver-notifier-builder"
WORKDIR /app
COPY . .
RUN GOOS=linux GOARCH=${TARGETARCH} go build

FROM debian:bullseye-slim
WORKDIR /app
RUN apt-get update && apt-get -y upgrade && apt-get -y install ca-certificates
COPY --from=builder /app/dggarchiver-notifier .
ENTRYPOINT [ "./dggarchiver-notifier" ]