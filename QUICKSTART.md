# Quick Start Guide

Get the Smart Home IoT system running in 5 minutes!

## Prerequisites

- Docker & Docker Compose installed
- Go 1.21+ (for microservices)
- Python 3.11+ (for simulator)

## Start in 3 Steps

### 1. Start Infrastructure

```bash
docker-compose up -d
```

Wait ~30 seconds for all services to initialize. Verify with:

```bash
docker-compose ps
```

All services should show "Up" status.

### 2. Start Core Services

**Terminal 1 - Device Registry Service:**
```bash
cd services/device-registry
go run main.go
```

**Terminal 2 - Data Ingestion Service:**
```bash
cd services/data-ingestion
go run main.go
```

### 3. Run Device Simulator

**Terminal 3 - Simulator:**
```bash
cd simulator
pip install -r requirements.txt
python simulator.py
```

## Verify It's Working

### Check Service Health

```bash
# Device Registry
curl http://localhost:8081/health

# Data Ingestion
curl http://localhost:8082/health
```

### View Device Telemetry (Live)

```bash
# Subscribe to all telemetry
mosquitto_sub -h localhost -t devices/+/telemetry -v
```

### View RabbitMQ Messages

1. Open http://localhost:15672 in browser
2. Login: `smarthome` / `smarthome_dev_password`
3. Go to Queues → `telemetry.raw` → Get Messages

### Check Database

```bash
# Connect to TimescaleDB
docker exec -it smarthome-timescaledb psql -U telemetry -d telemetry_db

# Query telemetry data
SELECT device_id, metric_name, metric_value, time
FROM device_telemetry
ORDER BY time DESC
LIMIT 10;
```

## Test the API

### Register a Device

```bash
curl -X POST http://localhost:8081/api/devices \
  -H "Content-Type: application/json" \
  -H "X-User-ID: test-user-123" \
  -d '{
    "name": "My Test Sensor",
    "type": "temperature_sensor",
    "location": "Office",
    "capabilities": {
      "metrics": ["temperature", "humidity"]
    }
  }'
```

### List All Devices

```bash
curl http://localhost:8081/api/devices
```

### Get Device Details

```bash
curl http://localhost:8081/api/devices/{device-id}
```

## Monitoring Dashboards

- **RabbitMQ**: http://localhost:15672 (smarthome / smarthome_dev_password)
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin / admin)

## What's Happening?

1. **Simulator** creates 14 virtual IoT devices
2. Devices send telemetry via **MQTT** every 5 seconds
3. **Data Ingestion Service** receives MQTT messages
4. Telemetry is stored in **TimescaleDB** (time-series database)
5. Messages are published to **RabbitMQ** for downstream processing
6. **Device Registry** manages device metadata

## Stop Everything

```bash
# Stop services (Ctrl+C in each terminal)

# Stop infrastructure
docker-compose down

# Stop and remove all data
docker-compose down -v
```

## Troubleshooting

### "Cannot connect to database"
Wait 10 more seconds - PostgreSQL is still initializing.

### "MQTT connection refused"
Check Mosquitto is running: `docker-compose logs mosquitto`

### "No telemetry in database"
1. Ensure Data Ingestion Service is running
2. Check logs for errors
3. Verify MQTT connection: `mosquitto_sub -h localhost -t test`

## Next Steps

- Read [GETTING_STARTED.md](GETTING_STARTED.md) for detailed documentation
- Explore [Architecture](docs/ARCHITECTURE.md) to understand the system
- Implement Rules Engine for automation
- Add Analytics Service for insights
- Build API Gateway for unified access

## Example Automation Rule (Coming Soon)

```json
{
  "name": "Turn on lights when motion detected",
  "conditions": {
    "all": [
      {"device": "motion-living-room", "metric": "motion_detected", "operator": "==", "value": true}
    ]
  },
  "actions": [
    {"device": "light-living-room", "command": "turn_on", "params": {"brightness": 80}}
  ]
}
```

---

**Need Help?** Check the README files in each service directory or open an issue!
