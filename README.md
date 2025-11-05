# Smart Home IoT Management System

A comprehensive microservices-based platform for managing and automating smart home IoT devices with real-time data processing, rules engine, and analytics.

## Architecture Overview

This system is built using a microservices architecture with the following services:

### Core Services

1. **Device Registry Service** (Go)
   - Register and manage IoT devices
   - Device metadata and capabilities
   - Multi-tenant device organization
   - REST API for device management

2. **Data Ingestion Service** (Go)
   - MQTT broker integration for device telemetry
   - High-throughput data ingestion
   - Data validation and enrichment
   - Publishes to message queue for downstream processing

3. **Rules Engine Service** (Python)
   - Define automation rules (IF-THEN-ELSE logic)
   - Real-time rule evaluation
   - Support for complex conditions and actions
   - Scene management (multiple actions grouped)

4. **Analytics Service** (Python)
   - Process device telemetry data
   - Generate insights and statistics
   - Anomaly detection
   - Historical data analysis

5. **Notification Service** (Node.js)
   - Multi-channel notifications (email, SMS, push)
   - Alert management
   - Template-based messaging
   - User preference handling

6. **User Management Service** (Go)
   - User authentication (JWT)
   - Multi-tenant support (homes/locations)
   - Authorization and permissions
   - User preferences

7. **API Gateway** (Go)
   - Single entry point for all services
   - Request routing and load balancing
   - Authentication middleware
   - Rate limiting

### Infrastructure Components

- **PostgreSQL**: Primary database for services
- **TimescaleDB**: Time-series data for device telemetry
- **Redis**: Caching and pub/sub
- **RabbitMQ**: Message broker for async communication
- **MQTT Broker (Mosquitto)**: IoT device communication
- **Prometheus + Grafana**: Monitoring and observability

## Technology Stack

- **Languages**: Go, Python, Node.js
- **Databases**: PostgreSQL, TimescaleDB, Redis
- **Message Broker**: RabbitMQ
- **IoT Protocol**: MQTT
- **Containerization**: Docker, Docker Compose
- **API**: REST, WebSocket

## Project Structure

```
smart-home-iot/
├── services/
│   ├── device-registry/       # Device management service (Go)
│   ├── data-ingestion/        # MQTT data ingestion (Go)
│   ├── rules-engine/          # Automation rules (Python)
│   ├── analytics/             # Data analytics (Python)
│   ├── notification/          # Notification service (Node.js)
│   ├── user-management/       # User and auth service (Go)
│   └── api-gateway/           # API Gateway (Go)
├── infrastructure/
│   ├── docker-compose.yml     # Infrastructure setup
│   └── config/                # Configuration files
├── shared/
│   ├── proto/                 # Protocol buffers (if using gRPC)
│   └── events/                # Event schemas
├── simulator/                 # IoT device simulator
└── docs/                      # Documentation
```

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Go 1.21+
- Python 3.11+
- Node.js 20+

### Quick Start

1. Start infrastructure services:
```bash
docker-compose up -d
```

2. Run services (each in separate terminal):
```bash
# Device Registry
cd services/device-registry && go run main.go

# Data Ingestion
cd services/data-ingestion && go run main.go

# Rules Engine
cd services/rules-engine && python main.py

# Analytics
cd services/analytics && python main.py

# Notification Service
cd services/notification && npm start

# API Gateway
cd services/api-gateway && go run main.go
```

3. Run IoT device simulator:
```bash
cd simulator && python simulator.py
```

## API Documentation

API documentation will be available at `http://localhost:8080/docs` when the API Gateway is running.

## Key Features

- **Real-time Device Monitoring**: Live telemetry data from IoT devices
- **Automation Rules**: Create complex automation rules with conditions and actions
- **Analytics Dashboard**: Historical data and insights
- **Multi-tenant Support**: Manage multiple homes/locations
- **Flexible Notifications**: Email, SMS, and push notifications
- **Device Simulator**: Test without real hardware

## Microservices Patterns Implemented

- **API Gateway Pattern**: Unified entry point
- **Event-Driven Architecture**: Async communication via RabbitMQ
- **Service Discovery**: Via Docker networking
- **Database per Service**: Each service owns its data
- **CQRS**: Separate read/write paths in analytics
- **Circuit Breaker**: Fault tolerance in service calls

## Development Roadmap

- [x] Project structure and architecture
- [x] Infrastructure setup (Docker Compose)
- [x] Device Registry Service (Go) ✅
- [x] Data Ingestion Service (Go) ✅
- [x] Rules Engine Service (Python) ✅
- [x] Analytics Service (Python) ✅
- [x] Notification Service (Node.js) ✅
- [x] User Management Service (Go) ✅
- [x] API Gateway (Go) ✅
- [x] Device Simulator (Python) ✅
- [x] Complete Testing Guide ✅
- [ ] Web Dashboard (Future)
- [ ] Mobile App (Future)

## License

MIT
