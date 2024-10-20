# Stage 1: Build the Go binary
FROM golang:1.23-alpine AS build

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the Go application
RUN go build -o app .

# Stage 2: Create a smaller image to run the application
FROM alpine:latest

# Set the working directory inside the container
WORKDIR /app

# Copy the built Go binary from the builder stage
COPY --from=build /app/app .

# Expose port (optional if your app serves HTTP, etc.)
#EXPOSE 1883

# Command to run the app
CMD ["./app"]

