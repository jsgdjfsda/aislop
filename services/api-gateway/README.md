# API Gateway

Unified entry point for all microservices with authentication, rate limiting, and request routing.

## Features

- **Single entry point** for all services
- **JWT authentication** middleware
- **Rate limiting** (100 requests/min per user, Redis-based)
- **Request routing** to appropriate backend services
- **Health checks** for all services
- **CORS handling**
- **Request/response logging**
- **Prometheus metrics**
- **Reverse proxy** for seamless service communication

## Architecture

```
Client → API Gateway → Backend Services
                ├─→ Device Registry (8081)
                ├─→ Data Ingestion (8082)
                ├─→ Rules Engine (8083)
                ├─→ Analytics (8084)
                ├─→ Notification (8085)
                └─→ User Management (8086)
```

## Routing Table

### Public Routes (No Auth)

| Route | Backend Service | Description |
|-------|----------------|-------------|
| `POST /api/auth/register` | User Management | Register new user |
| `POST /api/auth/login` | User Management | Login user |
| `POST /api/auth/refresh` | User Management | Refresh JWT token |

### Protected Routes (Auth Required)

| Route | Backend Service | Description |
|-------|----------------|-------------|
| `/api/devices/*` | Device Registry | Device CRUD operations |
| `/api/rules/*` | Rules Engine | Automation rules |
| `/api/analytics/*` | Analytics | Data analytics and insights |
| `/api/notifications/*` | Notification | Notification management |
| `/api/users/*` | User Management | User profile and homes |

## API Examples

### Register & Login

```bash
# Register
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password123","full_name":"John Doe"}'

# Login
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password123"}'

# Save the token from response
export TOKEN="eyJhbGciOiJIUzI1NiIs..."
```

### Device Management

```bash
# Create device
curl -X POST http://localhost:8080/api/devices \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Living Room Sensor","type":"temperature_sensor","location":"Living Room"}'

# List devices
curl http://localhost:8080/api/devices \
  -H "Authorization: Bearer $TOKEN"

# Get device
curl http://localhost:8080/api/devices/{device-id} \
  -H "Authorization: Bearer $TOKEN"
```

### Rules Management

```bash
# Create rule
curl -X POST http://localhost:8080/api/rules \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name":"Evening lights",
    "conditions":{"all":[{"device":"light-sensor-1","metric":"illuminance","operator":"<","value":100}]},
    "actions":[{"device":"light-1","command":"turn_on"}]
  }'

# List rules
curl http://localhost:8080/api/rules \
  -H "Authorization: Bearer $TOKEN"
```

### Analytics

```bash
# Device statistics
curl "http://localhost:8080/api/analytics/devices/temp-sensor-1/stats?metric=temperature&hours=24" \
  -H "Authorization: Bearer $TOKEN"

# Time-series data
curl "http://localhost:8080/api/analytics/devices/temp-sensor-1/timeseries?metric=temperature&hours=24" \
  -H "Authorization: Bearer $TOKEN"

# Anomaly detection
curl "http://localhost:8080/api/analytics/devices/temp-sensor-1/anomalies?metric=temperature" \
  -H "Authorization: Bearer $TOKEN"

# Energy consumption
curl "http://localhost:8080/api/analytics/energy?hours=24" \
  -H "Authorization: Bearer $TOKEN"
```

### Notifications

```bash
# Send notification
curl -X POST http://localhost:8080/api/notifications/send \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id":"user-123",
    "channels":["email"],
    "subject":"Test",
    "message":"Test notification"
  }'

# Get notification history
curl http://localhost:8080/api/notifications/history \
  -H "Authorization: Bearer $TOKEN"
```

## Rate Limiting

- **Limit:** 100 requests per minute per user/IP
- **Storage:** Redis
- **Response:** HTTP 429 Too Many Requests
- **Headers:**
  - `X-RateLimit-Limit: 100`
  - `X-RateLimit-Remaining: 75`

### Rate Limit Response

```json
{
  "error": "Rate limit exceeded",
  "retry_after": "60 seconds"
}
```

## Health Check

```bash
curl http://localhost:8080/health
```

**Response:**
```json
{
  "status": "healthy",
  "service": "api-gateway",
  "services": {
    "device-registry": true,
    "data-ingestion": true,
    "rules-engine": true,
    "analytics": true,
    "notification": true,
    "user-management": true
  }
}
```

## Authentication

The gateway validates JWT tokens for protected routes:

1. Client includes token in header: `Authorization: Bearer <token>`
2. Gateway validates token signature and expiry
3. Gateway extracts `user_id` and `email` from claims
4. Gateway adds headers to proxied request:
   - `X-User-ID: <user_id>`
   - `X-User-Email: <email>`
5. Backend services use these headers for authorization

## Configuration

### Environment Variables

```bash
API_GATEWAY_PORT=8080
JWT_SECRET=your-secret-key
REDIS_HOST=localhost
REDIS_PORT=6379

# Backend service URLs (defaults to localhost)
DEVICE_REGISTRY_URL=http://localhost:8081
RULES_ENGINE_URL=http://localhost:8083
ANALYTICS_URL=http://localhost:8084
NOTIFICATION_URL=http://localhost:8085
USER_MANAGEMENT_URL=http://localhost:8086
```

## Running

```bash
cd services/api-gateway
go mod download
go run main.go
```

Gateway starts on port **8080** by default.

## Metrics

Prometheus metrics at `/metrics`:

- `api_gateway_http_requests_total` - Total requests by service, method, status
- `api_gateway_http_duration_seconds` - Request latencies by service, method
- `api_gateway_rate_limit_exceeded_total` - Rate limit violations

## Features in Detail

### 1. Request Routing

Automatically routes requests to backend services based on URL path:
- Preserves original path and query parameters
- Forwards request body
- Handles HTTP methods (GET, POST, PUT, DELETE)

### 2. Authentication Propagation

Extracts user context from JWT and adds to headers:
```
Authorization: Bearer <token>  →  X-User-ID: user-123
                                  X-User-Email: user@example.com
```

### 3. Error Handling

- Service unavailable → HTTP 502 Bad Gateway
- Invalid token → HTTP 401 Unauthorized
- Rate limit exceeded → HTTP 429 Too Many Requests
- Service not found → HTTP 404 Not Found

### 4. CORS Support

Allows cross-origin requests for web clients:
- `Access-Control-Allow-Origin: *`
- Supports all common methods and headers
- Handles preflight OPTIONS requests

## Testing

### Test Authentication Flow

```bash
# 1. Register
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Test1234!"}'

# 2. Login (save token)
TOKEN=$(curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Test1234!"}' \
  | jq -r '.token')

# 3. Access protected route
curl http://localhost:8080/api/devices \
  -H "Authorization: Bearer $TOKEN"
```

### Test Rate Limiting

```bash
# Send 101 requests quickly (should trigger rate limit)
for i in {1..101}; do
  curl http://localhost:8080/api/devices \
    -H "Authorization: Bearer $TOKEN" \
    -w "\n%{http_code}\n"
done
```

### Test Service Health

```bash
curl http://localhost:8080/health | jq
```

## Troubleshooting

### 502 Bad Gateway

- Check if backend service is running
- Verify service URL configuration
- Check backend service logs

### 401 Unauthorized

- Verify JWT token is valid and not expired
- Check JWT_SECRET matches between gateway and user-management
- Ensure token is sent in Authorization header

### Rate Limit Issues

- Check Redis connection
- Verify Redis is running: `docker-compose ps redis`
- Rate limit resets every minute

## Production Considerations

1. **HTTPS:** Use reverse proxy (Nginx) for TLS termination
2. **JWT Secret:** Use strong, random secret (32+ characters)
3. **Rate Limiting:** Adjust limits based on use case
4. **Load Balancing:** Run multiple gateway instances
5. **Service Discovery:** Consider Consul/etcd for dynamic service URLs
6. **Circuit Breaker:** Add Hystrix/resilience4j for fault tolerance
7. **API Versioning:** Add `/api/v1/` prefix for versioning

## Future Enhancements

- [ ] Request/response transformation
- [ ] API key authentication (for IoT devices)
- [ ] WebSocket support
- [ ] GraphQL gateway
- [ ] Request caching
- [ ] API documentation (Swagger/OpenAPI)
- [ ] Advanced rate limiting (per endpoint)
- [ ] Service mesh integration (Istio)
