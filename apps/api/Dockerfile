# syntax=docker/dockerfile:1
FROM golang:1.27-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/lms ./cmd/lms

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/lms /lms
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/lms"]
CMD ["serve"]
