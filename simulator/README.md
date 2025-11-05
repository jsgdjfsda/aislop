# IoT Device Simulator

Simulates multiple IoT devices sending telemetry data via MQTT for testing the Smart Home system.

## Simulated Devices

The simulator creates the following virtual devices:

### Temperature Sensors
- Living Room, Bedroom, Kitchen
- Metrics: temperature (°C), humidity (%)

### Smart Lights
- Living Room, Bedroom, Kitchen, Hallway
- Metrics: state (on/off), brightness (0-100), power consumption (watts)

### Door/Window Sensors
- Front Door, Back Door, Bedroom Window
- Metrics: state (open/closed), battery level, time since last change

### Motion Sensors
- Living Room, Hallway
- Metrics: motion detected (boolean), time since last motion, battery level

### Smart Thermostat
- Main Floor
- Metrics: current temp, target temp, mode, heating/cooling status, humidity

### Energy Meter
- Main Electrical Panel
- Metrics: power (watts), voltage, current, power factor, frequency

## Features

- Realistic telemetry data with gradual changes
- Random events (lights toggling, doors opening, motion detection)
- MQTT publishing with proper message format
- Configurable update interval
- Auto-reconnect to MQTT broker

## Installation

```bash
cd simulator
pip install -r requirements.txt
```

## Usage

### Basic Usage

```bash
python simulator.py
```

This will:
1. Connect to MQTT broker (localhost:1883 by default)
2. Create 14 simulated devices
3. Publish telemetry every 5 seconds

### Configuration

Edit environment variables in `../.env`:

```bash
MQTT_BROKER=localhost
MQTT_PORT=1883
```

### MQTT Topics

Devices publish to:
```
devices/{device_id}/telemetry
```

Example:
```
devices/temp-sensor-living-room/telemetry
devices/light-bedroom/telemetry
devices/motion-hallway/telemetry
```

## Message Format

```json
{
  "device_id": "temp-sensor-living-room",
  "timestamp": "2025-11-05T12:00:00Z",
  "metrics": {
    "temperature": 22.5,
    "humidity": 45.0
  },
  "metadata": {
    "type": "temperature_sensor",
    "location": "Living Room"
  }
}
```

## Testing

### Subscribe to All Telemetry

```bash
mosquitto_sub -h localhost -t devices/+/telemetry -v
```

### Subscribe to Specific Device

```bash
mosquitto_sub -h localhost -t devices/temp-sensor-living-room/telemetry
```

### Monitor in RabbitMQ

1. Open RabbitMQ Management UI: http://localhost:15672
2. Login: smarthome / smarthome_dev_password
3. Go to Queues tab
4. Check `telemetry.raw` queue for messages

## Stopping the Simulator

Press `Ctrl+C` to gracefully stop the simulator.

## Customization

### Adding New Devices

Create a new device class:

```python
class MyCustomDevice(DeviceSimulator):
    def __init__(self, device_id: str, location: str):
        super().__init__(device_id, "custom_device", location)

    def generate_telemetry(self) -> Dict[str, Any]:
        return {
            "metric1": random.uniform(0, 100),
            "metric2": random.choice(["value1", "value2"])
        }
```

Add to `create_devices()` method:
```python
MyCustomDevice("my-device-1", "Location"),
```

### Adjusting Update Interval

Change the interval parameter:
```python
manager.run(interval=10)  # Update every 10 seconds
```

## Troubleshooting

### Cannot Connect to MQTT Broker

1. Ensure MQTT broker is running:
   ```bash
   docker-compose ps mosquitto
   ```

2. Check broker logs:
   ```bash
   docker-compose logs mosquitto
   ```

3. Test connection:
   ```bash
   mosquitto_pub -h localhost -t test -m "hello"
   ```

### No Data in TimescaleDB

1. Ensure Data Ingestion Service is running
2. Check service logs for errors
3. Verify TimescaleDB connection:
   ```bash
   docker exec -it smarthome-timescaledb psql -U telemetry -d telemetry_db
   SELECT COUNT(*) FROM device_telemetry;
   ```
