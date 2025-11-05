# Data Ingestion Service

High-throughput MQTT-based service for ingesting telemetry data from IoT devices.

## Features

- MQTT subscriber for device telemetry
- TimescaleDB storage for time-series data
- RabbitMQ publishing for downstream processing
- Auto-reconnect for MQTT and RabbitMQ
- Prometheus metrics
- Health checks

## MQTT Topics

### Telemetry Data
```
Topic: devices/{device_id}/telemetry
Payload:
{
  "device_id": "temp-sensor-1",
  "timestamp": "2025-11-05T12:00:00Z",
  "metrics": {
    "temperature": 22.5,
    "humidity": 45.0
  },
  "metadata": {
    "location": "Living Room"
  }
}
```

### Device Status
```
Topic: devices/{device_id}/status
Payload:
{
  "status": "online",
  "timestamp": "2025-11-05T12:00:00Z"
}
```

## Data Flow

1. Device publishes telemetry to MQTT broker
2. Data Ingestion Service receives message
3. Stores data in TimescaleDB (device_telemetry table)
4. Publishes to RabbitMQ exchange (device.telemetry)
5. Downstream services consume from RabbitMQ

## Running

```bash
cd services/data-ingestion
go mod download
go run main.go
```

HTTP server starts on port 8082 (health checks and metrics).

## Environment Variables

- `MQTT_BROKER` - MQTT broker host (default: localhost)
- `MQTT_PORT` - MQTT broker port (default: 1883)
- `TIMESCALE_HOST` - TimescaleDB host (default: localhost)
- `TIMESCALE_PORT` - TimescaleDB port (default: 5433)
- `RABBITMQ_HOST` - RabbitMQ host (default: localhost)
- `RABBITMQ_PORT` - RabbitMQ port (default: 5672)
- `DATA_INGESTION_PORT` - HTTP port (default: 8082)

## Metrics

Prometheus metrics available at `/metrics`:
- `data_ingestion_telemetry_received_total` - Total telemetry messages
- `data_ingestion_processing_duration_seconds` - Processing time
- `data_ingestion_mqtt_connected` - MQTT connection status

## Testing

Send test message via mosquitto_pub:

```bash
mosquitto_pub -h localhost -t devices/test-device/telemetry \
  -m '{"device_id":"test-device","metrics":{"temperature":22.5,"humidity":45}}'
```
