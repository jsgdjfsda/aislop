#!/usr/bin/env python3
"""
Rules Engine Service

Evaluates automation rules against incoming device telemetry.
Supports complex conditions (AND/OR) and multiple action types.
"""

import json
import logging
import os
import signal
import sys
import threading
import time
from datetime import datetime, time as dt_time
from typing import Dict, Any, List, Optional

import paho.mqtt.client as mqtt
import pika
import psycopg2
from dotenv import load_dotenv
from flask import Flask, request, jsonify
from prometheus_client import Counter, Histogram, Gauge, generate_latest

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Load environment variables
load_dotenv("../../.env")

# Configuration
POSTGRES_HOST = os.getenv("POSTGRES_HOST", "localhost")
POSTGRES_PORT = os.getenv("POSTGRES_PORT", "5432")
POSTGRES_USER = os.getenv("POSTGRES_USER", "smarthome")
POSTGRES_PASSWORD = os.getenv("POSTGRES_PASSWORD", "smarthome_dev_password")
RABBITMQ_HOST = os.getenv("RABBITMQ_HOST", "localhost")
RABBITMQ_PORT = int(os.getenv("RABBITMQ_PORT", "5672"))
RABBITMQ_USER = os.getenv("RABBITMQ_USER", "smarthome")
RABBITMQ_PASSWORD = os.getenv("RABBITMQ_PASSWORD", "smarthome_dev_password")
MQTT_BROKER = os.getenv("MQTT_BROKER", "localhost")
MQTT_PORT = int(os.getenv("MQTT_PORT", "1883"))
SERVICE_PORT = int(os.getenv("RULES_ENGINE_PORT", "8083"))

# Prometheus metrics
rules_evaluated = Counter('rules_engine_rules_evaluated_total', 'Total rules evaluated', ['rule_id', 'result'])
rules_execution_duration = Histogram('rules_engine_execution_duration_seconds', 'Rule execution time')
actions_executed = Counter('rules_engine_actions_executed_total', 'Total actions executed', ['action_type', 'status'])
active_rules = Gauge('rules_engine_active_rules', 'Number of active rules')


class RuleEvaluator:
    """Evaluates rule conditions against telemetry data"""

    OPERATORS = {
        '==': lambda a, b: a == b,
        '!=': lambda a, b: a != b,
        '>': lambda a, b: a > b,
        '<': lambda a, b: a < b,
        '>=': lambda a, b: a >= b,
        '<=': lambda a, b: a <= b,
        'in': lambda a, b: a in b,
        'not_in': lambda a, b: a not in b,
    }

    @staticmethod
    def evaluate_condition(condition: Dict[str, Any], telemetry: Dict[str, Any]) -> bool:
        """Evaluate a single condition"""
        try:
            # Device metric condition
            if 'device' in condition and 'metric' in condition:
                if telemetry.get('device_id') != condition['device']:
                    return False

                metric_value = telemetry.get('metrics', {}).get(condition['metric'])
                if metric_value is None:
                    return False

                operator = condition.get('operator', '==')
                expected_value = condition.get('value')

                op_func = RuleEvaluator.OPERATORS.get(operator)
                if not op_func:
                    logger.warning(f"Unknown operator: {operator}")
                    return False

                return op_func(metric_value, expected_value)

            # Time-based condition
            elif 'time' in condition:
                return RuleEvaluator.evaluate_time_condition(condition)

            # Day of week condition
            elif 'day_of_week' in condition:
                current_day = datetime.now().strftime('%A').lower()
                return current_day in [d.lower() for d in condition['day_of_week']]

            return False

        except Exception as e:
            logger.error(f"Error evaluating condition: {e}")
            return False

    @staticmethod
    def evaluate_time_condition(condition: Dict[str, Any]) -> bool:
        """Evaluate time-based conditions"""
        now = datetime.now().time()
        time_type = condition.get('time')
        value = condition.get('value')

        if time_type == 'after':
            target_time = datetime.strptime(value, '%H:%M').time()
            return now >= target_time
        elif time_type == 'before':
            target_time = datetime.strptime(value, '%H:%M').time()
            return now <= target_time
        elif time_type == 'between':
            start = datetime.strptime(value['start'], '%H:%M').time()
            end = datetime.strptime(value['end'], '%H:%M').time()
            return start <= now <= end

        return False

    @staticmethod
    def evaluate_conditions(conditions: Dict[str, Any], telemetry: Dict[str, Any]) -> bool:
        """Evaluate complex conditions (AND/OR logic)"""
        if 'all' in conditions:
            # AND logic - all conditions must be true
            return all(
                RuleEvaluator.evaluate_condition(cond, telemetry)
                for cond in conditions['all']
            )
        elif 'any' in conditions:
            # OR logic - at least one condition must be true
            return any(
                RuleEvaluator.evaluate_condition(cond, telemetry)
                for cond in conditions['any']
            )
        else:
            # Single condition
            return RuleEvaluator.evaluate_condition(conditions, telemetry)


class ActionExecutor:
    """Executes actions when rules match"""

    def __init__(self, mqtt_client: mqtt.Client, db_conn):
        self.mqtt_client = mqtt_client
        self.db_conn = db_conn

    def execute_actions(self, actions: List[Dict[str, Any]], rule_id: str, telemetry: Dict[str, Any]):
        """Execute all actions for a triggered rule"""
        for action in actions:
            try:
                if 'device' in action and 'command' in action:
                    self.execute_device_command(action)
                elif 'notify' in action:
                    self.execute_notification(action, rule_id)
                elif 'webhook' in action:
                    self.execute_webhook(action)
                else:
                    logger.warning(f"Unknown action type: {action}")

            except Exception as e:
                logger.error(f"Error executing action: {e}")
                actions_executed.labels(action_type='unknown', status='error').inc()

    def execute_device_command(self, action: Dict[str, Any]):
        """Send command to device via MQTT"""
        device_id = action['device']
        command = action['command']
        params = action.get('params', {})

        topic = f"devices/{device_id}/commands"
        payload = json.dumps({
            'command': command,
            'params': params,
            'timestamp': datetime.utcnow().isoformat() + 'Z'
        })

        result = self.mqtt_client.publish(topic, payload, qos=1)
        if result.rc == mqtt.MQTT_ERR_SUCCESS:
            logger.info(f"Sent command '{command}' to device {device_id}")
            actions_executed.labels(action_type='device_command', status='success').inc()
        else:
            logger.error(f"Failed to send command to {device_id}")
            actions_executed.labels(action_type='device_command', status='error').inc()

    def execute_notification(self, action: Dict[str, Any], rule_id: str):
        """Trigger notification (via RabbitMQ to notification service)"""
        # This would publish to RabbitMQ notification exchange
        logger.info(f"Notification triggered: {action.get('message')}")
        actions_executed.labels(action_type='notification', status='success').inc()

    def execute_webhook(self, action: Dict[str, Any]):
        """Call external webhook"""
        # This would make HTTP request to webhook URL
        logger.info(f"Webhook called: {action.get('webhook')}")
        actions_executed.labels(action_type='webhook', status='success').inc()


class RulesEngine:
    """Main rules engine service"""

    def __init__(self):
        self.db_conn = None
        self.mqtt_client = None
        self.rabbitmq_conn = None
        self.rabbitmq_channel = None
        self.evaluator = RuleEvaluator()
        self.executor = None
        self.rules_cache = {}
        self.running = False

    def connect_db(self):
        """Connect to PostgreSQL"""
        self.db_conn = psycopg2.connect(
            host=POSTGRES_HOST,
            port=POSTGRES_PORT,
            user=POSTGRES_USER,
            password=POSTGRES_PASSWORD,
            database='rules_engine_db'
        )
        logger.info("Connected to PostgreSQL")

    def connect_mqtt(self):
        """Connect to MQTT broker"""
        client_id = f"rules-engine-{int(time.time())}"
        self.mqtt_client = mqtt.Client(client_id)

        def on_connect(client, userdata, flags, rc):
            if rc == 0:
                logger.info("Connected to MQTT broker")
            else:
                logger.error(f"Failed to connect to MQTT broker: {rc}")

        self.mqtt_client.on_connect = on_connect
        self.mqtt_client.connect(MQTT_BROKER, MQTT_PORT, 60)
        self.mqtt_client.loop_start()

    def connect_rabbitmq(self):
        """Connect to RabbitMQ"""
        credentials = pika.PlainCredentials(RABBITMQ_USER, RABBITMQ_PASSWORD)
        parameters = pika.ConnectionParameters(
            host=RABBITMQ_HOST,
            port=RABBITMQ_PORT,
            credentials=credentials
        )
        self.rabbitmq_conn = pika.BlockingConnection(parameters)
        self.rabbitmq_channel = self.rabbitmq_conn.channel()

        # Declare exchange and queue
        self.rabbitmq_channel.exchange_declare(
            exchange='device.telemetry',
            exchange_type='topic',
            durable=True
        )

        result = self.rabbitmq_channel.queue_declare(queue='rules.telemetry', durable=True)
        queue_name = result.method.queue

        self.rabbitmq_channel.queue_bind(
            exchange='device.telemetry',
            queue=queue_name,
            routing_key='telemetry.#'
        )

        logger.info("Connected to RabbitMQ")

    def load_rules(self):
        """Load active rules from database"""
        cursor = self.db_conn.cursor()
        cursor.execute("""
            SELECT id, name, user_id, home_id, conditions, actions, enabled
            FROM rules
            WHERE enabled = true
        """)

        self.rules_cache = {}
        for row in cursor.fetchall():
            rule_id, name, user_id, home_id, conditions, actions, enabled = row
            self.rules_cache[rule_id] = {
                'id': rule_id,
                'name': name,
                'user_id': user_id,
                'home_id': home_id,
                'conditions': conditions,
                'actions': actions,
                'enabled': enabled
            }

        cursor.close()
        active_rules.set(len(self.rules_cache))
        logger.info(f"Loaded {len(self.rules_cache)} active rules")

    def process_telemetry(self, telemetry: Dict[str, Any]):
        """Process telemetry and evaluate rules"""
        with rules_execution_duration.time():
            for rule_id, rule in self.rules_cache.items():
                try:
                    # Evaluate rule conditions
                    matched = self.evaluator.evaluate_conditions(
                        rule['conditions'],
                        telemetry
                    )

                    if matched:
                        logger.info(f"Rule '{rule['name']}' matched for device {telemetry.get('device_id')}")
                        rules_evaluated.labels(rule_id=rule_id, result='matched').inc()

                        # Execute actions
                        self.executor.execute_actions(
                            rule['actions'],
                            rule_id,
                            telemetry
                        )
                    else:
                        rules_evaluated.labels(rule_id=rule_id, result='not_matched').inc()

                except Exception as e:
                    logger.error(f"Error processing rule {rule_id}: {e}")
                    rules_evaluated.labels(rule_id=rule_id, result='error').inc()

    def consume_telemetry(self):
        """Consume telemetry from RabbitMQ"""
        def callback(ch, method, properties, body):
            try:
                telemetry = json.loads(body)
                self.process_telemetry(telemetry)
                ch.basic_ack(delivery_tag=method.delivery_tag)
            except Exception as e:
                logger.error(f"Error processing message: {e}")
                ch.basic_nack(delivery_tag=method.delivery_tag)

        self.rabbitmq_channel.basic_qos(prefetch_count=10)
        self.rabbitmq_channel.basic_consume(
            queue='rules.telemetry',
            on_message_callback=callback
        )

        logger.info("Started consuming telemetry from RabbitMQ")
        self.rabbitmq_channel.start_consuming()

    def start(self):
        """Start the rules engine"""
        self.running = True

        # Connect to services
        self.connect_db()
        self.connect_mqtt()
        self.connect_rabbitmq()

        # Initialize executor
        self.executor = ActionExecutor(self.mqtt_client, self.db_conn)

        # Load rules
        self.load_rules()

        # Start consuming in separate thread
        consumer_thread = threading.Thread(target=self.consume_telemetry)
        consumer_thread.daemon = True
        consumer_thread.start()

        # Periodically reload rules
        def reload_rules():
            while self.running:
                time.sleep(60)  # Reload every minute
                try:
                    self.load_rules()
                except Exception as e:
                    logger.error(f"Error reloading rules: {e}")

        reload_thread = threading.Thread(target=reload_rules)
        reload_thread.daemon = True
        reload_thread.start()

        logger.info("Rules Engine started successfully")

    def stop(self):
        """Stop the rules engine"""
        self.running = False

        if self.rabbitmq_channel:
            self.rabbitmq_channel.stop_consuming()
        if self.rabbitmq_conn:
            self.rabbitmq_conn.close()
        if self.mqtt_client:
            self.mqtt_client.loop_stop()
            self.mqtt_client.disconnect()
        if self.db_conn:
            self.db_conn.close()

        logger.info("Rules Engine stopped")


# Flask REST API
app = Flask(__name__)
engine = None


@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint"""
    return jsonify({'status': 'healthy', 'service': 'rules-engine'}), 200


@app.route('/metrics', methods=['GET'])
def metrics():
    """Prometheus metrics endpoint"""
    return generate_latest(), 200


@app.route('/api/rules', methods=['GET'])
def list_rules():
    """List all rules"""
    try:
        cursor = engine.db_conn.cursor()
        user_id = request.headers.get('X-User-ID')

        query = "SELECT id, name, conditions, actions, enabled, created_at FROM rules"
        params = []

        if user_id:
            query += " WHERE user_id = %s"
            params.append(user_id)

        query += " ORDER BY created_at DESC"

        cursor.execute(query, params)
        rules = []

        for row in cursor.fetchall():
            rules.append({
                'id': row[0],
                'name': row[1],
                'conditions': row[2],
                'actions': row[3],
                'enabled': row[4],
                'created_at': row[5].isoformat()
            })

        cursor.close()
        return jsonify({'rules': rules, 'count': len(rules)}), 200

    except Exception as e:
        logger.error(f"Error listing rules: {e}")
        return jsonify({'error': str(e)}), 500


@app.route('/api/rules', methods=['POST'])
def create_rule():
    """Create a new rule"""
    try:
        data = request.get_json()
        user_id = request.headers.get('X-User-ID', 'default-user')

        cursor = engine.db_conn.cursor()
        cursor.execute("""
            INSERT INTO rules (name, user_id, home_id, conditions, actions, enabled)
            VALUES (%s, %s, %s, %s, %s, %s)
            RETURNING id, created_at
        """, (
            data['name'],
            user_id,
            data.get('home_id', '00000000-0000-0000-0000-000000000000'),
            json.dumps(data['conditions']),
            json.dumps(data['actions']),
            data.get('enabled', True)
        ))

        rule_id, created_at = cursor.fetchone()
        engine.db_conn.commit()
        cursor.close()

        # Reload rules cache
        engine.load_rules()

        return jsonify({
            'id': rule_id,
            'message': 'Rule created successfully',
            'created_at': created_at.isoformat()
        }), 201

    except Exception as e:
        logger.error(f"Error creating rule: {e}")
        return jsonify({'error': str(e)}), 500


@app.route('/api/rules/<rule_id>', methods=['PUT'])
def update_rule(rule_id):
    """Update a rule"""
    try:
        data = request.get_json()
        cursor = engine.db_conn.cursor()

        cursor.execute("""
            UPDATE rules
            SET name = %s, conditions = %s, actions = %s, enabled = %s, updated_at = NOW()
            WHERE id = %s
        """, (
            data.get('name'),
            json.dumps(data.get('conditions')),
            json.dumps(data.get('actions')),
            data.get('enabled'),
            rule_id
        ))

        engine.db_conn.commit()
        cursor.close()

        # Reload rules cache
        engine.load_rules()

        return jsonify({'message': 'Rule updated successfully'}), 200

    except Exception as e:
        logger.error(f"Error updating rule: {e}")
        return jsonify({'error': str(e)}), 500


@app.route('/api/rules/<rule_id>', methods=['DELETE'])
def delete_rule(rule_id):
    """Delete a rule"""
    try:
        cursor = engine.db_conn.cursor()
        cursor.execute("DELETE FROM rules WHERE id = %s", (rule_id,))
        engine.db_conn.commit()
        cursor.close()

        # Reload rules cache
        engine.load_rules()

        return jsonify({'message': 'Rule deleted successfully'}), 200

    except Exception as e:
        logger.error(f"Error deleting rule: {e}")
        return jsonify({'error': str(e)}), 500


def run_flask_app():
    """Run Flask app in separate thread"""
    app.run(host='0.0.0.0', port=SERVICE_PORT, debug=False, use_reloader=False)


def signal_handler(sig, frame):
    """Handle shutdown signals"""
    logger.info("Shutdown signal received")
    if engine:
        engine.stop()
    sys.exit(0)


def main():
    """Main entry point"""
    global engine

    logger.info("Starting Rules Engine Service")

    # Initialize engine
    engine = RulesEngine()
    engine.start()

    # Start Flask API in separate thread
    flask_thread = threading.Thread(target=run_flask_app)
    flask_thread.daemon = True
    flask_thread.start()

    # Setup signal handlers
    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    # Keep main thread alive
    while True:
        time.sleep(1)


if __name__ == "__main__":
    main()
