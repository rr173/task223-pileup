FROM docker.m.daocloud.io/library/golang:1.26.3-bookworm

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app/pileup ./cmd/pileup

EXPOSE 8080
ENTRYPOINT ["/app/pileup"]
CMD ["--smoke-test"]
