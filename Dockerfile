FROM golang:1.17-alpine as builder
ARG APP
WORKDIR /app
ENV GOPROXY=https://goproxy.cn
COPY ./go.mod ./
COPY ./go.sum ./
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o ${APP} ./cmd/${APP}

FROM busybox as runner
ARG APP
COPY --from=builder /app/${APP} /app
ENTRYPOINT ["/app"]
