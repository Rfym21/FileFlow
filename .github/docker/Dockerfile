FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

ARG TARGETARCH
COPY ${TARGETARCH}/fileflow ./fileflow

EXPOSE 8080
CMD ["./fileflow"]
