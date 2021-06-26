FROM golang:1.15-alpine3.12 as builder
ARG GOPROXY
ARG APP
ENV GOPROXY=${GOPROXY}
WORKDIR /go/pixiu
COPY . .
RUN CGO_ENABLED=0 go build -a -o ./dist/${APP} cmd/${APP}/${APP}.go

FROM jacky06/static:nonroot
ARG APP
WORKDIR /
COPY --from=builder /go/pixiu/dist/${APP} /usr/local/bin/${APP}
USER root:root
