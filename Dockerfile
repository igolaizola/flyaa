# builder image
FROM golang:alpine AS builder
ARG TARGETPLATFORM
COPY . /src
WORKDIR /src
RUN apk add --no-cache make bash git
RUN make app-build PLATFORMS=$TARGETPLATFORM

# running image
FROM alpine
ARG BASE_URL
ENV FLYAA_BASE_URL=$BASE_URL
WORKDIR /home
COPY --from=builder /src/bin/flyaa-* /bin/flyaa

# executable
ENTRYPOINT [ "/bin/flyaa" ]
