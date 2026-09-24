FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -p=2 -trimpath -ldflags="-s -w" -o /golottery ./cmd/golottery

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /golottery /golottery
EXPOSE 5568
USER nonroot:nonroot
ENTRYPOINT ["/golottery"]
