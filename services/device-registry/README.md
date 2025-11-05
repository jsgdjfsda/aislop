# Device Registry Service

REST API service for managing IoT device registration and metadata.

## Features

- Device CRUD operations
- Device authentication token generation
- Multi-tenant support
- Device status tracking
- Prometheus metrics
- Health checks

## API Endpoints

### Health Check
```
GET /health
```

### Create Device
```
POST /api/devices
Content-Type: application/json
X-User-ID: <user-id>

{
  "name": "Living Room Temp Sensor",
  "type": "temperature_sensor",
  "location": "Living Room",
  "capabilities": {
    "metrics": ["temperature", "humidity"]
  }
}
```

### List Devices
```
GET /api/devices?type=temperature_sensor&status=online
X-User-ID: <user-id>
```

### Get Device
```
GET /api/devices/:id
```

### Update Device
```
PUT /api/devices/:id
Content-Type: application/json

{
  "name": "Updated Name",
  "location": "New Location"
}
```

### Delete Device
```
DELETE /api/devices/:id
```

### Update Device Status
```
POST /api/devices/:id/status
Content-Type: application/json

{
  "status": "online"
}
```

## Running

```bash
cd services/device-registry
go mod download
go run main.go
```

Service starts on port 8081 by default.

## Environment Variables

- `POSTGRES_HOST` - PostgreSQL host (default: localhost)
- `POSTGRES_PORT` - PostgreSQL port (default: 5432)
- `POSTGRES_USER` - Database user
- `POSTGRES_PASSWORD` - Database password
- `DEVICE_REGISTRY_PORT` - Service port (default: 8081)

## Metrics

Prometheus metrics available at `/metrics`:
- `device_registry_http_requests_total` - Total HTTP requests
- `device_registry_http_request_duration_seconds` - Request latencies
- `device_registry_devices_total` - Total devices registered
