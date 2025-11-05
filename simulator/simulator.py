#!/usr/bin/env python3
"""
IoT Device Simulator

Simulates multiple IoT devices sending telemetry data via MQTT.
Supports various device types: temperature sensors, smart lights, door locks, etc.
"""

import json
import random
import time
import logging
from datetime import datetime
from typing import Dict, Any
import os
import paho.mqtt.client as mqtt
from dotenv import load_dotenv

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Load environment variables
load_dotenv("../.env")

MQTT_BROKER = os.getenv("MQTT_BROKER", "localhost")
MQTT_PORT = int(os.getenv("MQTT_PORT", "1883"))


class DeviceSimulator:
    """Base class for device simulators"""

    def __init__(self, device_id: str, device_type: str, location: str):
        self.device_id = device_id
        self.device_type = device_type
        self.location = location

    def generate_telemetry(self) -> Dict[str, Any]:
        """Generate telemetry data (override in subclass)"""
        raise NotImplementedError

    def to_mqtt_message(self) -> Dict[str, Any]:
        """Convert to MQTT message format"""
        return {
            "device_id": self.device_id,
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "metrics": self.generate_telemetry(),
            "metadata": {
                "type": self.device_type,
                "location": self.location
            }
        }


class TemperatureSensor(DeviceSimulator):
    """Temperature and humidity sensor"""

    def __init__(self, device_id: str, location: str):
        super().__init__(device_id, "temperature_sensor", location)
        self.base_temp = random.uniform(18, 24)
        self.base_humidity = random.uniform(30, 60)

    def generate_telemetry(self) -> Dict[str, float]:
        # Simulate gradual changes
        self.base_temp += random.uniform(-0.5, 0.5)
        self.base_humidity += random.uniform(-2, 2)

        # Keep within realistic bounds
        self.base_temp = max(15, min(30, self.base_temp))
        self.base_humidity = max(20, min(80, self.base_humidity))

        return {
            "temperature": round(self.base_temp, 2),
            "humidity": round(self.base_humidity, 2)
        }


class SmartLight(DeviceSimulator):
    """Smart light bulb"""

    def __init__(self, device_id: str, location: str):
        super().__init__(device_id, "smart_light", location)
        self.is_on = random.choice([True, False])
        self.brightness = random.randint(0, 100) if self.is_on else 0
        self.power_consumption = 0

    def generate_telemetry(self) -> Dict[str, Any]:
        # Randomly toggle light
        if random.random() < 0.1:
            self.is_on = not self.is_on
            if self.is_on:
                self.brightness = random.randint(50, 100)
            else:
                self.brightness = 0

        # Adjust brightness occasionally
        if self.is_on and random.random() < 0.2:
            self.brightness = random.randint(20, 100)

        # Calculate power consumption (approximate)
        self.power_consumption = (self.brightness / 100) * 10 if self.is_on else 0

        return {
            "state": "on" if self.is_on else "off",
            "brightness": self.brightness,
            "power_watts": round(self.power_consumption, 2)
        }


class DoorSensor(DeviceSimulator):
    """Door/window sensor"""

    def __init__(self, device_id: str, location: str):
        super().__init__(device_id, "door_sensor", location)
        self.is_open = False
        self.last_change = time.time()

    def generate_telemetry(self) -> Dict[str, Any]:
        # Randomly open/close door
        if random.random() < 0.05:  # 5% chance per reading
            self.is_open = not self.is_open
            self.last_change = time.time()

        seconds_since_change = int(time.time() - self.last_change)

        return {
            "state": "open" if self.is_open else "closed",
            "battery_level": random.randint(80, 100),
            "seconds_since_change": seconds_since_change
        }


class MotionSensor(DeviceSimulator):
    """Motion/occupancy sensor"""

    def __init__(self, device_id: str, location: str):
        super().__init__(device_id, "motion_sensor", location)
        self.motion_detected = False
        self.last_motion = time.time() - random.randint(0, 300)

    def generate_telemetry(self) -> Dict[str, Any]:
        # Simulate motion detection
        if random.random() < 0.15:  # 15% chance
            self.motion_detected = True
            self.last_motion = time.time()
        else:
            # Motion clears after 10 seconds
            if time.time() - self.last_motion > 10:
                self.motion_detected = False

        seconds_since_motion = int(time.time() - self.last_motion)

        return {
            "motion_detected": self.motion_detected,
            "seconds_since_motion": seconds_since_motion,
            "battery_level": random.randint(75, 100)
        }


class SmartThermostat(DeviceSimulator):
    """Smart thermostat/HVAC controller"""

    def __init__(self, device_id: str, location: str):
        super().__init__(device_id, "smart_thermostat", location)
        self.target_temp = 22.0
        self.current_temp = random.uniform(20, 24)
        self.mode = "auto"  # auto, heat, cool, off
        self.is_heating = False
        self.is_cooling = False

    def generate_telemetry(self) -> Dict[str, Any]:
        # Simulate temperature changes
        if self.is_heating:
            self.current_temp += random.uniform(0.1, 0.3)
        elif self.is_cooling:
            self.current_temp -= random.uniform(0.1, 0.3)
        else:
            self.current_temp += random.uniform(-0.2, 0.2)

        # Control logic
        if self.mode == "auto":
            if self.current_temp < self.target_temp - 0.5:
                self.is_heating = True
                self.is_cooling = False
            elif self.current_temp > self.target_temp + 0.5:
                self.is_heating = False
                self.is_cooling = True
            else:
                self.is_heating = False
                self.is_cooling = False

        # Occasionally change target temp
        if random.random() < 0.05:
            self.target_temp = random.uniform(20, 24)

        return {
            "current_temperature": round(self.current_temp, 2),
            "target_temperature": round(self.target_temp, 2),
            "mode": self.mode,
            "is_heating": self.is_heating,
            "is_cooling": self.is_cooling,
            "humidity": random.randint(35, 55)
        }


class EnergyMeter(DeviceSimulator):
    """Energy consumption meter"""

    def __init__(self, device_id: str, location: str):
        super().__init__(device_id, "energy_meter", location)
        self.base_power = random.uniform(500, 1500)

    def generate_telemetry(self) -> Dict[str, float]:
        # Simulate varying power consumption
        power = self.base_power + random.uniform(-200, 200)
        power = max(0, power)

        voltage = 230 + random.uniform(-5, 5)
        current = power / voltage

        return {
            "power_watts": round(power, 2),
            "voltage": round(voltage, 2),
            "current_amps": round(current, 2),
            "power_factor": round(random.uniform(0.85, 0.95), 2),
            "frequency_hz": round(50 + random.uniform(-0.1, 0.1), 2)
        }


class SimulatorManager:
    """Manages multiple device simulators"""

    def __init__(self, mqtt_broker: str, mqtt_port: int):
        self.mqtt_broker = mqtt_broker
        self.mqtt_port = mqtt_port
        self.client = None
        self.devices = []
        self.running = False

    def connect_mqtt(self):
        """Connect to MQTT broker"""
        client_id = f"simulator-{int(time.time())}"
        self.client = mqtt.Client(client_id)

        def on_connect(client, userdata, flags, rc):
            if rc == 0:
                logger.info("Connected to MQTT broker")
            else:
                logger.error(f"Failed to connect to MQTT broker, code: {rc}")

        def on_disconnect(client, userdata, rc):
            logger.warning("Disconnected from MQTT broker")

        self.client.on_connect = on_connect
        self.client.on_disconnect = on_disconnect

        try:
            self.client.connect(self.mqtt_broker, self.mqtt_port, 60)
            self.client.loop_start()
        except Exception as e:
            logger.error(f"Error connecting to MQTT broker: {e}")
            raise

    def create_devices(self):
        """Create simulated devices"""
        self.devices = [
            # Temperature sensors
            TemperatureSensor("temp-sensor-living-room", "Living Room"),
            TemperatureSensor("temp-sensor-bedroom", "Bedroom"),
            TemperatureSensor("temp-sensor-kitchen", "Kitchen"),

            # Smart lights
            SmartLight("light-living-room", "Living Room"),
            SmartLight("light-bedroom", "Bedroom"),
            SmartLight("light-kitchen", "Kitchen"),
            SmartLight("light-hallway", "Hallway"),

            # Door sensors
            DoorSensor("door-front", "Front Door"),
            DoorSensor("door-back", "Back Door"),
            DoorSensor("window-bedroom", "Bedroom Window"),

            # Motion sensors
            MotionSensor("motion-living-room", "Living Room"),
            MotionSensor("motion-hallway", "Hallway"),

            # Thermostat
            SmartThermostat("thermostat-main", "Main Floor"),

            # Energy meter
            EnergyMeter("energy-meter-main", "Electrical Panel"),
        ]

        logger.info(f"Created {len(self.devices)} simulated devices")

    def publish_telemetry(self, device: DeviceSimulator):
        """Publish device telemetry to MQTT"""
        try:
            message = device.to_mqtt_message()
            topic = f"devices/{device.device_id}/telemetry"
            payload = json.dumps(message)

            result = self.client.publish(topic, payload, qos=1)
            if result.rc == mqtt.MQTT_ERR_SUCCESS:
                logger.debug(f"Published telemetry for {device.device_id}")
            else:
                logger.warning(f"Failed to publish for {device.device_id}")

        except Exception as e:
            logger.error(f"Error publishing telemetry for {device.device_id}: {e}")

    def run(self, interval: int = 5):
        """Run simulator loop"""
        self.running = True
        logger.info(f"Starting simulator (publishing every {interval} seconds)")

        try:
            while self.running:
                for device in self.devices:
                    self.publish_telemetry(device)

                time.sleep(interval)

        except KeyboardInterrupt:
            logger.info("Simulator stopped by user")
        finally:
            self.stop()

    def stop(self):
        """Stop simulator"""
        self.running = False
        if self.client:
            self.client.loop_stop()
            self.client.disconnect()
        logger.info("Simulator stopped")


def main():
    """Main entry point"""
    logger.info(f"IoT Device Simulator")
    logger.info(f"MQTT Broker: {MQTT_BROKER}:{MQTT_PORT}")

    # Create simulator manager
    manager = SimulatorManager(MQTT_BROKER, MQTT_PORT)

    # Connect to MQTT
    manager.connect_mqtt()

    # Create devices
    manager.create_devices()

    # Run simulator
    manager.run(interval=5)


if __name__ == "__main__":
    main()
