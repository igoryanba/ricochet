# Multi-stage build for Ricochet
FROM golang:1.23-bookworm AS builder

# Install build dependencies for whisper.cpp
RUN apt-get update && apt-get install -y \
    cmake \
    build-essential \
    git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 1. Build whisper.cpp
COPY third_party/whisper.cpp ./third_party/whisper.cpp
WORKDIR /app/third_party/whisper.cpp
RUN cmake -B build -DWHISPER_COREML=0 -DWHISPER_OPENBLAS=0 && \
    cmake --build build --config Release -j$(nproc)

# 2. Build Ricochet
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o ricochet ./cmd/ricochet

# Final Stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binaries and models
COPY --from=builder /app/ricochet .
COPY --from=builder /app/third_party/whisper.cpp/build/bin/whisper-cli ./whisper-cli
COPY third_party/whisper.cpp/models/ggml-base.bin ./models/ggml-base.bin

# Create tmp directory for audio processing
RUN mkdir -p /root/.ricochet/tmp && chmod 777 /root/.ricochet/tmp

# Expose state directory as volume
VOLUME ["/root/.ricochet"]

# Healthcheck to ensure bot is running
# Note: Since it's a bridge, we just check if the process is alive for now
HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
    CMD ps aux | grep ricochet || exit 1

# Set paths for Ricochet
# (can be overridden by Env vars if implemented, 
# but for now we'll adjust main.go to check for these paths)
ENV WHISPER_PATH=/app/whisper-cli
ENV WHISPER_MODEL_PATH=/app/models/ggml-base.bin

ENTRYPOINT ["./ricochet"]
