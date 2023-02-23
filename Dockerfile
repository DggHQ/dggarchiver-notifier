FROM golang:bullseye as builder
LABEL builder=true multistage_tag="dggarchiver-notifier-builder"
WORKDIR /app
COPY . .
RUN GOOS=linux GOARCH=amd64 go build

FROM debian:bullseye-slim
WORKDIR /app
RUN apt-get update && apt-get -y upgrade && apt-get -y install ca-certificates
COPY --from=builder /app/dggarchiver-notifier .
ENTRYPOINT [ "./dggarchiver-notifier" ]