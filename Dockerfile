# ==============================================================================
# Production Dockerfile for Go API Gateway (Pre-compiled Linux Binary)
# ==============================================================================
FROM alpine:3.20

WORKDIR /app

# Install minimal runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl && \
    addgroup -g 1000 -S gateway && \
    adduser -u 1000 -S gateway -G gateway

# Copy pre-compiled Linux binary and static web dashboard
COPY api-gateway-linux /app/api-gateway
COPY web /app/web

# Permissions
RUN chown -R gateway:gateway /app && chmod +x /app/api-gateway

USER gateway

ENV PORT=8080 \
    REDIS_ADDR=redis:6379 \
    ENVIRONMENT=production

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=3s --retries=3 \
    CMD curl -f http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/api-gateway"]
