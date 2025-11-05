# Getting Started with Smart Home IoT Platform

This guide will help you set up and run the Smart Home IoT Management System on your local machine.

## Prerequisites

- **Docker** and **Docker Compose** (v2.0+)
- **Go** 1.21 or higher
- **Python** 3.11 or higher
- **Node.js** 20 or higher
- **Git**

## Quick Start

### 1. Clone the Repository

```bash
git clone <repository-url>
cd aislop
```

### 2. Start Infrastructure Services

Start all required infrastructure services (PostgreSQL, TimescaleDB, Redis, RabbitMQ, MQTT Broker):

```bash
docker-compose up -d
```

Verify all services are running:

```bash
docker-compose ps
```

You should see the following services running:
- `smarthome-postgres` (PostgreSQL on port 5432)
- `smarthome-timescaledb` (TimescaleDB on port 5433)
- `smarthome-redis` (Redis on port 6379)
- `smarthome-rabbitmq` (RabbitMQ on ports 5672, 15672)
- `smarthome-mosquitto` (MQTT on ports 1883, 9001)
- `smarthome-prometheus` (Prometheus on port 9090)
- `smarthome-grafana` (Grafana on port 3000)

### 3. Configure Environment Variables

Copy the example environment file:

```bash
cp .env.example .env
```

Edit `.env` if needed (default values work for local development).

### 4. Run Microservices

Each service can be run independently. Open separate terminal windows for each.

#### Device Registry Service (Go)

```bash
cd services/device-registry
go mod download
go run main.go
```

Service will start on `http://localhost:8081`

#### Data Ingestion Service (Go)

```bash
cd services/data-ingestion
go mod download
go run main.go
```

Service will start on `http://localhost:8082`

#### Rules Engine Service (Python)

```bash
cd services/rules-engine
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
pip install -r requirements.txt
python main.py
```

Service will start on `http://localhost:8083`

#### Analytics Service (Python)

```bash
cd services/analytics
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python main.py
```

Service will start on `http://localhost:8084`

#### Notification Service (Node.js)

```bash
cd services/notification
npm install
npm start
```

Service will start on `http://localhost:8085`

#### User Management Service (Go)

```bash
cd services/user-management
go mod download
go run main.go
```

Service will start on `http://localhost:8086`

#### API Gateway (Go)

```bash
cd services/api-gateway
go mod download
go run main.go
```

Service will start on `http://localhost:8080`

### 5. Run IoT Device Simulator

To test the system without real IoT devices:

```bash
cd simulator
pip install -r requirements.txt
python simulator.py
```

The simulator will create virtual devices and send telemetry data.

## Accessing Services

### API Gateway
- **URL**: http://localhost:8080
- **Health Check**: http://localhost:8080/health

### RabbitMQ Management UI
- **URL**: http://localhost:15672
- **Username**: smarthome
- **Password**: smarthome_dev_password

### Prometheus
- **URL**: http://localhost:9090

### Grafana
- **URL**: http://localhost:3000
- **Username**: admin
- **Password**: admin

## Testing the System

### 1. Register a User

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123",
    "full_name": "John Doe"
  }'
```

### 2. Login

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'
```

Save the JWT token from the response.

### 3. Register a Device

```bash
curl -X POST http://localhost:8080/api/devices \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-jwt-token>" \
  -d '{
    "name": "Living Room Temperature Sensor",
    "type": "temperature_sensor",
    "location": "Living Room",
    "capabilities": {
      "metrics": ["temperature", "humidity"]
    }
  }'
```

### 4. View Devices

```bash
curl http://localhost:8080/api/devices \
  -H "Authorization: Bearer <your-jwt-token>"
```

### 5. Create an Automation Rule

```bash
curl -X POST http://localhost:8080/api/rules \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-jwt-token>" \
  -d '{
    "name": "Turn on heater when cold",
    "conditions": {
      "all": [
        {
          "device": "temp-sensor-id",
          "metric": "temperature",
          "operator": "<",
          "value": 18
        }
      ]
    },
    "actions": [
      {
        "device": "heater-id",
        "command": "turn_on"
      }
    ]
  }'
```

## Development Workflow

### Running Tests

Each service has its own tests:

```bash
# Go services
cd services/device-registry
go test ./...

# Python services
cd services/rules-engine
pytest

# Node.js services
cd services/notification
npm test
```

### Viewing Logs

View logs from infrastructure services:

```bash
docker-compose logs -f [service-name]
```

Examples:
```bash
docker-compose logs -f mosquitto
docker-compose logs -f rabbitmq
```

### Stopping Services

Stop all infrastructure services:

```bash
docker-compose down
```

Stop and remove volumes (⚠️ deletes all data):

```bash
docker-compose down -v
```

## Troubleshooting

### Database Connection Issues

Ensure PostgreSQL is ready:
```bash
docker-compose logs postgres
```

Wait for the message: "database system is ready to accept connections"

### MQTT Connection Issues

Test MQTT broker:
```bash
# Subscribe to test topic
docker exec -it smarthome-mosquitto mosquitto_sub -h localhost -t test

# In another terminal, publish
docker exec -it smarthome-mosquitto mosquitto_pub -h localhost -t test -m "hello"
```

### RabbitMQ Issues

Check RabbitMQ management UI at http://localhost:15672

View queues and exchanges to verify message flow.

### Port Conflicts

If ports are already in use, modify `docker-compose.yml` to use different ports.

## Next Steps

- Check out the [Architecture Documentation](docs/ARCHITECTURE.md)
- Review the [API Documentation](docs/API.md) (coming soon)
- Explore example use cases in `examples/` (coming soon)
- Set up monitoring with Grafana dashboards

## Need Help?

- Check the [FAQ](docs/FAQ.md) (coming soon)
- Review service-specific README files in each service directory
- Open an issue on GitHub
