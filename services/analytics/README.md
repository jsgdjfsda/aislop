# Analytics Service

Advanced analytics and insights service for device telemetry data with ML-based anomaly detection.

## Features

- Device statistics (average, min, max, std deviation)
- Time-series data aggregation
- Anomaly detection using Isolation Forest
- Energy consumption tracking and cost estimation
- Device uptime/availability monitoring
- Trend analysis
- Real-time query performance metrics

## API Endpoints

### Device Statistics

Get statistical summary for a device metric:

```bash
GET /api/analytics/devices/{device_id}/stats?metric=temperature&hours=24
```

**Response:**
```json
{
  "device_id": "temp-sensor-living-room",
  "metric_name": "temperature",
  "period_hours": 24,
  "statistics": {
    "average": 22.5,
    "minimum": 18.2,
    "maximum": 25.8,
    "std_deviation": 1.3,
    "data_points": 288
  }
}
```

### Time-Series Data

Get aggregated time-series data:

```bash
GET /api/analytics/devices/{device_id}/timeseries?metric=temperature&hours=24&interval=1 hour
```

**Parameters:**
- `metric` - Metric name (default: temperature)
- `hours` - Time period (default: 24)
- `interval` - Aggregation interval: "5 minutes", "1 hour", "1 day" (default: 1 hour)

**Response:**
```json
{
  "device_id": "temp-sensor-living-room",
  "metric_name": "temperature",
  "interval": "1 hour",
  "data_points": [
    {
      "timestamp": "2025-11-05T12:00:00",
      "average": 22.5,
      "minimum": 22.0,
      "maximum": 23.0
    },
    ...
  ]
}
```

### Anomaly Detection

Detect anomalies using ML-based Isolation Forest:

```bash
GET /api/analytics/devices/{device_id}/anomalies?metric=temperature&hours=168&contamination=0.1
```

**Parameters:**
- `metric` - Metric name
- `hours` - Analysis period (default: 168 = 1 week)
- `contamination` - Expected % of anomalies (default: 0.1 = 10%)

**Response:**
```json
{
  "device_id": "temp-sensor-living-room",
  "metric_name": "temperature",
  "period_hours": 168,
  "anomalies_detected": 5,
  "anomalies": [
    {
      "timestamp": "2025-11-05T03:15:00",
      "value": 35.2,
      "score": -0.15
    },
    ...
  ]
}
```

### Energy Consumption

Calculate energy usage and cost:

```bash
# Single device
GET /api/analytics/energy?device_id=energy-meter-main&hours=24

# All devices
GET /api/analytics/energy?hours=24
```

**Response:**
```json
{
  "period_hours": 24,
  "devices": [
    {
      "device_id": "energy-meter-main",
      "average_watts": 1250.5,
      "peak_watts": 2100.0,
      "energy_kwh": 30.012
    }
  ],
  "total_energy_kwh": 30.012,
  "estimated_cost_usd": 3.60
}
```

### Device Uptime

Calculate device availability:

```bash
GET /api/analytics/devices/{device_id}/uptime?days=7
```

**Response:**
```json
{
  "device_id": "temp-sensor-living-room",
  "period_days": 7,
  "uptime_percentage": 98.5,
  "active_intervals": 1980,
  "total_intervals": 2016
}
```

### Trend Analysis

Analyze trends using statistical methods:

```bash
GET /api/analytics/devices/{device_id}/trend?metric=temperature&hours=168
```

**Response:**
```json
{
  "device_id": "temp-sensor-living-room",
  "metric_name": "temperature",
  "period_hours": 168,
  "trend": "increasing",
  "slope": 0.0012,
  "data_points": 336
}
```

**Trend values:** `increasing`, `decreasing`, `stable`, `insufficient_data`

### Analytics Summary

Get overall system summary:

```bash
GET /api/analytics/summary
```

**Response:**
```json
{
  "last_24_hours": {
    "total_data_points": 4032,
    "active_devices": 14
  }
}
```

## Running

```bash
cd services/analytics
pip install -r requirements.txt
python main.py
```

Service starts on port **8084** by default.

## Environment Variables

- `TIMESCALE_HOST` - TimescaleDB host (default: localhost)
- `TIMESCALE_PORT` - TimescaleDB port (default: 5433)
- `TIMESCALE_USER` - Database user
- `TIMESCALE_PASSWORD` - Database password
- `TIMESCALE_DB` - Database name (default: telemetry_db)
- `ANALYTICS_PORT` - Service port (default: 8084)

## Use Cases

### 1. Temperature Monitoring Dashboard

```bash
# Get current statistics
curl "http://localhost:8084/api/analytics/devices/temp-sensor-living-room/stats?metric=temperature&hours=1"

# Get hourly averages for chart
curl "http://localhost:8084/api/analytics/devices/temp-sensor-living-room/timeseries?metric=temperature&hours=24&interval=1 hour"
```

### 2. Detect Unusual Activity

```bash
# Check for temperature anomalies
curl "http://localhost:8084/api/analytics/devices/temp-sensor-living-room/anomalies?metric=temperature&hours=168"

# Check for unusual power consumption
curl "http://localhost:8084/api/analytics/devices/energy-meter-main/anomalies?metric=power_watts&hours=168"
```

### 3. Energy Cost Analysis

```bash
# Daily energy report
curl "http://localhost:8084/api/analytics/energy?hours=24"

# Weekly energy report
curl "http://localhost:8084/api/analytics/energy?hours=168"

# Specific device energy usage
curl "http://localhost:8084/api/analytics/energy?device_id=smart-light-living-room&hours=24"
```

### 4. Device Health Monitoring

```bash
# Check device uptime
curl "http://localhost:8084/api/analytics/devices/temp-sensor-bedroom/uptime?days=30"

# Analyze temperature stability
curl "http://localhost:8084/api/analytics/devices/temp-sensor-bedroom/trend?metric=temperature&hours=168"
```

## Advanced Examples

### Multi-Device Comparison

Get statistics for multiple devices:

```bash
# Living room temperature
curl "http://localhost:8084/api/analytics/devices/temp-sensor-living-room/stats?metric=temperature&hours=24"

# Bedroom temperature
curl "http://localhost:8084/api/analytics/devices/temp-sensor-bedroom/stats?metric=temperature&hours=24"

# Kitchen temperature
curl "http://localhost:8084/api/analytics/devices/temp-sensor-kitchen/stats?metric=temperature&hours=24"
```

### Weekly Energy Report

```python
import requests

response = requests.get(
    'http://localhost:8084/api/analytics/energy',
    params={'hours': 168}
)

data = response.json()
print(f"Total weekly consumption: {data['total_energy_kwh']} kWh")
print(f"Estimated cost: ${data['estimated_cost_usd']}")

for device in data['devices']:
    print(f"{device['device_id']}: {device['energy_kwh']} kWh")
```

### Anomaly Alert System

```python
import requests

response = requests.get(
    'http://localhost:8084/api/analytics/devices/temp-sensor-living-room/anomalies',
    params={'metric': 'temperature', 'hours': 24}
)

data = response.json()

if data['anomalies_detected'] > 0:
    print(f"⚠️ {data['anomalies_detected']} anomalies detected!")
    for anomaly in data['anomalies']:
        print(f"  - {anomaly['timestamp']}: {anomaly['value']}°C")
```

## Machine Learning

### Anomaly Detection Algorithm

The service uses **Isolation Forest**, an unsupervised ML algorithm that:

1. Isolates observations by randomly selecting a feature
2. Randomly selecting a split value between min and max
3. Anomalies are easier to isolate (fewer splits needed)
4. Returns anomaly score for each data point

**Contamination parameter** controls sensitivity:
- `0.01` = Very strict (1% expected anomalies)
- `0.1` = Moderate (10% expected anomalies)  ← Default
- `0.2` = Lenient (20% expected anomalies)

### Customizing Detection

```bash
# Strict detection (catch only extreme anomalies)
curl "http://localhost:8084/api/analytics/devices/temp-sensor-1/anomalies?contamination=0.01"

# Lenient detection (catch more potential issues)
curl "http://localhost:8084/api/analytics/devices/temp-sensor-1/anomalies?contamination=0.2"
```

## Performance

### Query Optimization

TimescaleDB features used for performance:

1. **Hypertables**: Automatic time-based partitioning
2. **Continuous Aggregates**: Pre-computed summaries
3. **Indexes**: Fast lookups by device_id and time
4. **time_bucket()**: Efficient time-series aggregation

### Caching Strategy

Consider adding Redis caching for:
- Device statistics (cache for 5 minutes)
- Energy reports (cache for 15 minutes)
- Trend analysis (cache for 1 hour)

## Metrics

Prometheus metrics at `/metrics`:

- `analytics_queries_total` - Total queries by endpoint and status
- `analytics_query_duration_seconds` - Query execution time by endpoint

## Testing

### Test Statistics

```bash
curl "http://localhost:8084/api/analytics/devices/temp-sensor-living-room/stats?metric=temperature&hours=1"
```

### Test Anomaly Detection

```bash
# Ensure you have at least 1 week of data
curl "http://localhost:8084/api/analytics/devices/temp-sensor-living-room/anomalies?metric=temperature&hours=168"
```

### Test Energy Calculation

```bash
curl "http://localhost:8084/api/analytics/energy?hours=24"
```

## Troubleshooting

### "No data available"

- Ensure Data Ingestion Service is running
- Verify devices are sending telemetry
- Check TimescaleDB has data:
  ```sql
  SELECT COUNT(*) FROM device_telemetry WHERE device_id = 'temp-sensor-living-room';
  ```

### Anomaly detection returns empty list

- Need minimum 10 data points
- Try increasing the hours parameter
- Check if device has been reporting data

### Slow queries

- Check database indexes: `\d device_telemetry`
- Reduce time range (hours parameter)
- Use continuous aggregates for large datasets

## Future Enhancements

- [ ] Predictive maintenance using LSTM
- [ ] Seasonal decomposition (STL)
- [ ] Correlation analysis between devices
- [ ] Real-time streaming analytics
- [ ] Custom alert thresholds
- [ ] Export reports as PDF/CSV
