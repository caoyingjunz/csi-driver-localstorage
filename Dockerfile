FROM golang:1.17-alpine as builder
ARG APP
WORKDIR /app
ENV GOPROXY=https://goproxy.cn
COPY ./go.mod ./
COPY ./go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o ${APP} ./cmd/${APP}

FROM busybox as runner
ARG APP
COPY --from=builder /app/${APP} /app
ENTRYPOINT ["/app"]
