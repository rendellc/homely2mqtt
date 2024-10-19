package main

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"os/signal"
	"rendellc/homely2mqtt/homely"
	"rendellc/homely2mqtt/mqtt"
	"slices"
	"strings"
	"time"
)

type deviceDescriptor struct {
	nameMQTT     string
	sensorType   sensorType
	locationMQTT *string
	floor        *string
	floorMQTT    *string
	room         *string
	roomMQTT     *string
	device       *homely.Device
}

var deviceLookupTable map[string]*deviceDescriptor

func lookupDevice(home *homely.Home, deviceID string) (*deviceDescriptor, error) {
	if deviceLookupTable == nil {
		deviceLookupTable = make(map[string]*deviceDescriptor)
	}

	// TODO: is this even more efficient than just looping over the devices every time?
	dev, found := deviceLookupTable[deviceID]
	if found {
		return dev, nil
	}

	i := slices.IndexFunc(home.Devices, func(d homely.Device) bool {
		if d.ID == deviceID {
			return true
		} else {
			return false
		}
	})
	if i < 0 {
		return nil, fmt.Errorf("cant find deviceID %s in %v", deviceID, home)
	}
	descriptor := createDeviceDescriptor(&home.Devices[i])
	deviceLookupTable[deviceID] = &descriptor

	return deviceLookupTable[deviceID], nil
}

func nameToMqtt(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "_")
}

type sensorType string

const (
	motionSensor  sensorType = "motion"
	smokeSensor              = "smoke"
	entrySensor              = "entry"
	unknownSensor            = "unknown"
)

func createDeviceDescriptor(device *homely.Device) deviceDescriptor {
	desc := deviceDescriptor{}
	switch {
	case strings.Contains(device.ModelName, "Motion"): // Alarm Motion Sensor 2
		desc.sensorType = motionSensor
	case strings.Contains(device.ModelName, "Smoke"): // Intelligent Smoke Alarm
		desc.sensorType = smokeSensor
	case strings.Contains(device.ModelName, "Entry"): // Alarm Entry Sensor 2
		desc.sensorType = entrySensor
	default:
		desc.sensorType = unknownSensor
	}

	parts := strings.Split(device.Location, " - ") // Floor 0 - Entrance
	if len(parts) == 2 {
		desc.floor = &parts[0]
		desc.room = &parts[1]
	} else {
		desc.floor = nil
		desc.room = nil
	}

	if desc.floor != nil && desc.room != nil {
		desc.locationMQTT = new(string)
		*desc.locationMQTT = nameToMqtt(device.Location)
	}
	if desc.floor != nil {
		desc.floorMQTT = new(string)
		*desc.floorMQTT = nameToMqtt(*desc.floor)
	}
	if desc.room != nil {
		desc.roomMQTT = new(string)
		*desc.roomMQTT = nameToMqtt(*desc.room)
	}

	desc.nameMQTT = nameToMqtt(device.Name)
	desc.device = device

	return desc
}

// Lookup environment variable, if it is not present
// then crash the application
func requireEnvVar(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		log.Fatalf("can't find required environment variable %s", name)
	}

	return value
}

func main() {
	mqttBrokerUrl := requireEnvVar("MQTT_BROKER_URL")
	mqttClientName := requireEnvVar("MQTT_CLIENT_NAME")
	homelyUser := requireEnvVar("HOMELY_USER")
	homelyPwd := requireEnvVar("HOMELY_PASSWORD")

	mqttClientID := fmt.Sprintf("%s_%d", mqttClientName, rand.IntN(1000))

	log.Printf("creating homely api client")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	homelyClient := homely.NewClient(homelyUser, homelyPwd)

	mqttClient := mqtt.NewClient(mqttBrokerUrl, mqttClientID, "/home/homely")
	err := mqttClient.Connect()
	if err != nil {
		log.Fatalf("cant connect to mqtt broker: %s", err.Error())
	}
	defer mqttClient.Disconnect()

	log.Printf("Connected to MQTT broker and Homely API")

	location, err := homelyClient.GetLocation(ctx)
	if err != nil {
		log.Fatalf("cannot get location data from homely: %s", err.Error())
	}
	time.Sleep(500 * time.Millisecond) // to avoid hitting API again instantly
	home, err := homelyClient.GetHome(ctx, location.LocationID)
	if err != nil {
		log.Fatalf("cannot get home data from homely: %s", err.Error())
	}

	pubHome := func(name string, value any) {
		err := mqttClient.Publish(fmt.Sprintf("home/%s", name), value, true)
		if err != nil {
			log.Printf("unable to publish home value: %s", err.Error())
		}
	}
	pubAlarm := func(state string) { pubHome("alarm", state) }

	pubDevice := func(deviceID string, valueName string, value any) {
		descriptor, err := lookupDevice(home, deviceID)
		if err != nil {
			log.Printf("unable to lookup device: %s", err.Error())
			return
		}

		d := descriptor
		_ = mqttClient.Publish(fmt.Sprintf("location/%s/%s/%s", *d.locationMQTT, d.nameMQTT, valueName), value, true)
		_ = mqttClient.Publish(fmt.Sprintf("device/%s/%s", d.nameMQTT, valueName), value, true)
		_ = mqttClient.Publish(fmt.Sprintf("%s/%s", deviceID, valueName), value, true)
	}

	for _, device := range home.Devices {
		descriptor, err := lookupDevice(home, device.ID)
		if err != nil {
			log.Printf("error describing home device: %s", err.Error())
			continue
		}

		d := descriptor
		_ = mqttClient.Publish(fmt.Sprintf("device/%s/%s", d.nameMQTT, "id"), d.device.ID, true)
		_ = mqttClient.Publish(fmt.Sprintf("device/%s/%s", d.nameMQTT, "sensor"), d.sensorType, true)
		_ = mqttClient.Publish(fmt.Sprintf("device/%s/%s", d.nameMQTT, "location"), device.Location, true)
		if d.floor != nil {
			_ = mqttClient.Publish(fmt.Sprintf("device/%s/%s", d.nameMQTT, "floor"), *d.floor, true)
		}
		if d.room != nil {
			_ = mqttClient.Publish(fmt.Sprintf("device/%s/%s", d.nameMQTT, "room"), *d.room, true)
		}
	}

	pubAlarm(home.AlarmState)

	// MQTT and Homely clients are both connected
	deviceChanged := func(ev homely.EventDeviceStateChanged) {
		for _, change := range ev.Changes {
			pubDevice(ev.DeviceID,
				change.StateName,
				change.Value)
		}
	}
	alarmChanged := func(ev homely.EventAlarmStateChanged) {
		home.AlarmState = ev.State
		pubAlarm(home.AlarmState)
	}

	go homelyClient.ConnectHome(ctx, home.LocationID, deviceChanged, alarmChanged)

	for {
		select {
		case <-ctx.Done():
			return
		}
	}
}
