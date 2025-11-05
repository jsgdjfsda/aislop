#!/usr/bin/env python3
"""
Analytics Service

Provides analytics and insights on device telemetry data.
Includes statistics, anomaly detection, trends, and energy tracking.
"""

import logging
import os
from datetime import datetime, timedelta
from typing import Dict, Any, List, Optional

import numpy as np
import pandas as pd
import psycopg2
from dotenv import load_dotenv
from flask import Flask, request, jsonify
from prometheus_client import Counter, Histogram, generate_latest
from sklearn.ensemble import IsolationForest

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Load environment variables
load_dotenv("../../.env")

# Configuration
TIMESCALE_HOST = os.getenv("TIMESCALE_HOST", "localhost")
TIMESCALE_PORT = os.getenv("TIMESCALE_PORT", "5433")
TIMESCALE_USER = os.getenv("TIMESCALE_USER", "telemetry")
TIMESCALE_PASSWORD = os.getenv("TIMESCALE_PASSWORD", "telemetry_dev_password")
TIMESCALE_DB = os.getenv("TIMESCALE_DB", "telemetry_db")
SERVICE_PORT = int(os.getenv("ANALYTICS_PORT", "8084"))

# Prometheus metrics
analytics_queries = Counter('analytics_queries_total', 'Total analytics queries', ['endpoint', 'status'])
query_duration = Histogram('analytics_query_duration_seconds', 'Query execution time', ['endpoint'])

# Flask app
app = Flask(__name__)
db_conn = None


def get_db_connection():
    """Get database connection"""
    global db_conn
    if db_conn is None or db_conn.closed:
        db_conn = psycopg2.connect(
            host=TIMESCALE_HOST,
            port=TIMESCALE_PORT,
            user=TIMESCALE_USER,
            password=TIMESCALE_PASSWORD,
            database=TIMESCALE_DB
        )
        logger.info("Connected to TimescaleDB")
    return db_conn


class DeviceAnalytics:
    """Analytics calculations for device data"""

    @staticmethod
    def get_device_statistics(device_id: str, metric_name: str, hours: int = 24) -> Dict[str, Any]:
        """Get statistical summary for a device metric"""
        conn = get_db_connection()
        cursor = conn.cursor()

        start_time = datetime.utcnow() - timedelta(hours=hours)

        query = """
            SELECT
                AVG(metric_value) as avg_value,
                MIN(metric_value) as min_value,
                MAX(metric_value) as max_value,
                STDDEV(metric_value) as std_dev,
                COUNT(*) as count
            FROM device_telemetry
            WHERE device_id = %s
              AND metric_name = %s
              AND time >= %s
        """

        cursor.execute(query, (device_id, metric_name, start_time))
        row = cursor.fetchone()
        cursor.close()

        if row and row[4] > 0:  # Check count > 0
            return {
                'device_id': device_id,
                'metric_name': metric_name,
                'period_hours': hours,
                'statistics': {
                    'average': float(row[0]) if row[0] else 0,
                    'minimum': float(row[1]) if row[1] else 0,
                    'maximum': float(row[2]) if row[2] else 0,
                    'std_deviation': float(row[3]) if row[3] else 0,
                    'data_points': int(row[4])
                }
            }

        return {
            'device_id': device_id,
            'metric_name': metric_name,
            'period_hours': hours,
            'statistics': None,
            'message': 'No data available for the specified period'
        }

    @staticmethod
    def get_time_series(device_id: str, metric_name: str, hours: int = 24, interval: str = '1 hour') -> List[Dict]:
        """Get time-series data with aggregation"""
        conn = get_db_connection()
        cursor = conn.cursor()

        start_time = datetime.utcnow() - timedelta(hours=hours)

        query = f"""
            SELECT
                time_bucket(%s, time) as bucket,
                AVG(metric_value) as avg_value,
                MIN(metric_value) as min_value,
                MAX(metric_value) as max_value
            FROM device_telemetry
            WHERE device_id = %s
              AND metric_name = %s
              AND time >= %s
            GROUP BY bucket
            ORDER BY bucket DESC
        """

        cursor.execute(query, (interval, device_id, metric_name, start_time))

        data_points = []
        for row in cursor.fetchall():
            data_points.append({
                'timestamp': row[0].isoformat(),
                'average': float(row[1]) if row[1] else 0,
                'minimum': float(row[2]) if row[2] else 0,
                'maximum': float(row[3]) if row[3] else 0
            })

        cursor.close()

        return data_points

    @staticmethod
    def detect_anomalies(device_id: str, metric_name: str, hours: int = 168, contamination: float = 0.1) -> List[Dict]:
        """Detect anomalies using Isolation Forest"""
        conn = get_db_connection()
        cursor = conn.cursor()

        start_time = datetime.utcnow() - timedelta(hours=hours)

        query = """
            SELECT time, metric_value
            FROM device_telemetry
            WHERE device_id = %s
              AND metric_name = %s
              AND time >= %s
            ORDER BY time ASC
        """

        cursor.execute(query, (device_id, metric_name, start_time))
        rows = cursor.fetchall()
        cursor.close()

        if len(rows) < 10:  # Need minimum data points
            return []

        # Prepare data
        timestamps = [row[0] for row in rows]
        values = np.array([row[1] for row in rows]).reshape(-1, 1)

        # Train Isolation Forest
        model = IsolationForest(contamination=contamination, random_state=42)
        predictions = model.fit_predict(values)

        # Extract anomalies
        anomalies = []
        for i, pred in enumerate(predictions):
            if pred == -1:  # -1 indicates anomaly
                anomalies.append({
                    'timestamp': timestamps[i].isoformat(),
                    'value': float(values[i][0]),
                    'score': float(model.score_samples(values[i].reshape(1, -1))[0])
                })

        return anomalies

    @staticmethod
    def get_energy_consumption(device_id: Optional[str] = None, hours: int = 24) -> Dict[str, Any]:
        """Calculate energy consumption from power metrics"""
        conn = get_db_connection()
        cursor = conn.cursor()

        start_time = datetime.utcnow() - timedelta(hours=hours)

        # Query for power metrics
        if device_id:
            query = """
                SELECT
                    device_id,
                    AVG(metric_value) as avg_watts,
                    MAX(metric_value) as peak_watts
                FROM device_telemetry
                WHERE device_id = %s
                  AND metric_name IN ('power_watts', 'power')
                  AND time >= %s
                GROUP BY device_id
            """
            cursor.execute(query, (device_id, start_time))
        else:
            query = """
                SELECT
                    device_id,
                    AVG(metric_value) as avg_watts,
                    MAX(metric_value) as peak_watts
                FROM device_telemetry
                WHERE metric_name IN ('power_watts', 'power')
                  AND time >= %s
                GROUP BY device_id
            """
            cursor.execute(query, (start_time,))

        results = []
        total_kwh = 0

        for row in cursor.fetchall():
            avg_watts = float(row[1]) if row[1] else 0
            peak_watts = float(row[2]) if row[2] else 0
            kwh = (avg_watts * hours) / 1000  # Convert to kWh

            results.append({
                'device_id': row[0],
                'average_watts': avg_watts,
                'peak_watts': peak_watts,
                'energy_kwh': round(kwh, 3)
            })

            total_kwh += kwh

        cursor.close()

        return {
            'period_hours': hours,
            'devices': results,
            'total_energy_kwh': round(total_kwh, 3),
            'estimated_cost_usd': round(total_kwh * 0.12, 2)  # Assuming $0.12 per kWh
        }

    @staticmethod
    def get_device_uptime(device_id: str, days: int = 7) -> Dict[str, Any]:
        """Calculate device uptime/availability"""
        conn = get_db_connection()
        cursor = conn.cursor()

        start_time = datetime.utcnow() - timedelta(days=days)

        # Count time buckets with data
        query = """
            SELECT
                COUNT(DISTINCT time_bucket('5 minutes', time)) as active_buckets
            FROM device_telemetry
            WHERE device_id = %s
              AND time >= %s
        """

        cursor.execute(query, (device_id, start_time))
        active_buckets = cursor.fetchone()[0] or 0

        # Total possible buckets (5-minute intervals)
        total_buckets = (days * 24 * 60) // 5
        uptime_percentage = (active_buckets / total_buckets * 100) if total_buckets > 0 else 0

        cursor.close()

        return {
            'device_id': device_id,
            'period_days': days,
            'uptime_percentage': round(uptime_percentage, 2),
            'active_intervals': active_buckets,
            'total_intervals': total_buckets
        }

    @staticmethod
    def get_trend_analysis(device_id: str, metric_name: str, hours: int = 168) -> Dict[str, Any]:
        """Analyze trends using linear regression"""
        conn = get_db_connection()
        cursor = conn.cursor()

        start_time = datetime.utcnow() - timedelta(hours=hours)

        query = """
            SELECT time, metric_value
            FROM device_telemetry
            WHERE device_id = %s
              AND metric_name = %s
              AND time >= %s
            ORDER BY time ASC
        """

        cursor.execute(query, (device_id, metric_name, start_time))
        rows = cursor.fetchall()
        cursor.close()

        if len(rows) < 2:
            return {'trend': 'insufficient_data'}

        # Convert to pandas for analysis
        df = pd.DataFrame(rows, columns=['timestamp', 'value'])
        df['timestamp_numeric'] = pd.to_datetime(df['timestamp']).astype(int) // 10**9

        # Simple linear regression
        x = df['timestamp_numeric'].values
        y = df['value'].values

        # Calculate trend
        coefficients = np.polyfit(x, y, 1)
        slope = coefficients[0]

        # Determine trend direction
        if abs(slope) < 1e-8:
            trend = 'stable'
        elif slope > 0:
            trend = 'increasing'
        else:
            trend = 'decreasing'

        return {
            'device_id': device_id,
            'metric_name': metric_name,
            'period_hours': hours,
            'trend': trend,
            'slope': float(slope),
            'data_points': len(rows)
        }


# API Endpoints

@app.route('/health', methods=['GET'])
def health():
    """Health check"""
    return jsonify({'status': 'healthy', 'service': 'analytics'}), 200


@app.route('/metrics', methods=['GET'])
def metrics():
    """Prometheus metrics"""
    return generate_latest(), 200


@app.route('/api/analytics/devices/<device_id>/stats', methods=['GET'])
def device_statistics(device_id):
    """Get device statistics"""
    try:
        with query_duration.labels(endpoint='device_stats').time():
            metric_name = request.args.get('metric', 'temperature')
            hours = int(request.args.get('hours', 24))

            stats = DeviceAnalytics.get_device_statistics(device_id, metric_name, hours)
            analytics_queries.labels(endpoint='device_stats', status='success').inc()

            return jsonify(stats), 200

    except Exception as e:
        logger.error(f"Error getting device statistics: {e}")
        analytics_queries.labels(endpoint='device_stats', status='error').inc()
        return jsonify({'error': str(e)}), 500


@app.route('/api/analytics/devices/<device_id>/timeseries', methods=['GET'])
def device_timeseries(device_id):
    """Get time-series data"""
    try:
        with query_duration.labels(endpoint='timeseries').time():
            metric_name = request.args.get('metric', 'temperature')
            hours = int(request.args.get('hours', 24))
            interval = request.args.get('interval', '1 hour')

            data = DeviceAnalytics.get_time_series(device_id, metric_name, hours, interval)
            analytics_queries.labels(endpoint='timeseries', status='success').inc()

            return jsonify({
                'device_id': device_id,
                'metric_name': metric_name,
                'interval': interval,
                'data_points': data
            }), 200

    except Exception as e:
        logger.error(f"Error getting time-series: {e}")
        analytics_queries.labels(endpoint='timeseries', status='error').inc()
        return jsonify({'error': str(e)}), 500


@app.route('/api/analytics/devices/<device_id>/anomalies', methods=['GET'])
def device_anomalies(device_id):
    """Detect anomalies"""
    try:
        with query_duration.labels(endpoint='anomalies').time():
            metric_name = request.args.get('metric', 'temperature')
            hours = int(request.args.get('hours', 168))
            contamination = float(request.args.get('contamination', 0.1))

            anomalies = DeviceAnalytics.detect_anomalies(device_id, metric_name, hours, contamination)
            analytics_queries.labels(endpoint='anomalies', status='success').inc()

            return jsonify({
                'device_id': device_id,
                'metric_name': metric_name,
                'period_hours': hours,
                'anomalies_detected': len(anomalies),
                'anomalies': anomalies
            }), 200

    except Exception as e:
        logger.error(f"Error detecting anomalies: {e}")
        analytics_queries.labels(endpoint='anomalies', status='error').inc()
        return jsonify({'error': str(e)}), 500


@app.route('/api/analytics/energy', methods=['GET'])
def energy_consumption():
    """Get energy consumption"""
    try:
        with query_duration.labels(endpoint='energy').time():
            device_id = request.args.get('device_id')
            hours = int(request.args.get('hours', 24))

            data = DeviceAnalytics.get_energy_consumption(device_id, hours)
            analytics_queries.labels(endpoint='energy', status='success').inc()

            return jsonify(data), 200

    except Exception as e:
        logger.error(f"Error calculating energy consumption: {e}")
        analytics_queries.labels(endpoint='energy', status='error').inc()
        return jsonify({'error': str(e)}), 500


@app.route('/api/analytics/devices/<device_id>/uptime', methods=['GET'])
def device_uptime(device_id):
    """Get device uptime"""
    try:
        with query_duration.labels(endpoint='uptime').time():
            days = int(request.args.get('days', 7))

            data = DeviceAnalytics.get_device_uptime(device_id, days)
            analytics_queries.labels(endpoint='uptime', status='success').inc()

            return jsonify(data), 200

    except Exception as e:
        logger.error(f"Error calculating uptime: {e}")
        analytics_queries.labels(endpoint='uptime', status='error').inc()
        return jsonify({'error': str(e)}), 500


@app.route('/api/analytics/devices/<device_id>/trend', methods=['GET'])
def device_trend(device_id):
    """Get trend analysis"""
    try:
        with query_duration.labels(endpoint='trend').time():
            metric_name = request.args.get('metric', 'temperature')
            hours = int(request.args.get('hours', 168))

            data = DeviceAnalytics.get_trend_analysis(device_id, metric_name, hours)
            analytics_queries.labels(endpoint='trend', status='success').inc()

            return jsonify(data), 200

    except Exception as e:
        logger.error(f"Error analyzing trend: {e}")
        analytics_queries.labels(endpoint='trend', status='error').inc()
        return jsonify({'error': str(e)}), 500


@app.route('/api/analytics/summary', methods=['GET'])
def analytics_summary():
    """Get overall analytics summary"""
    try:
        conn = get_db_connection()
        cursor = conn.cursor()

        # Get total data points in last 24 hours
        query = """
            SELECT COUNT(*) FROM device_telemetry
            WHERE time >= NOW() - INTERVAL '24 hours'
        """
        cursor.execute(query)
        total_points = cursor.fetchone()[0]

        # Get unique devices reporting
        query = """
            SELECT COUNT(DISTINCT device_id) FROM device_telemetry
            WHERE time >= NOW() - INTERVAL '24 hours'
        """
        cursor.execute(query)
        active_devices = cursor.fetchone()[0]

        cursor.close()

        return jsonify({
            'last_24_hours': {
                'total_data_points': total_points,
                'active_devices': active_devices
            }
        }), 200

    except Exception as e:
        logger.error(f"Error getting summary: {e}")
        return jsonify({'error': str(e)}), 500


def main():
    """Main entry point"""
    logger.info("Starting Analytics Service")

    # Test database connection
    try:
        get_db_connection()
        logger.info("Database connection successful")
    except Exception as e:
        logger.error(f"Failed to connect to database: {e}")
        return

    # Start Flask app
    logger.info(f"Analytics Service starting on port {SERVICE_PORT}")
    app.run(host='0.0.0.0', port=SERVICE_PORT, debug=False)


if __name__ == "__main__":
    main()
