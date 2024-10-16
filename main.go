package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"rendellc/homely2mqtt/homely"
	"rendellc/homely2mqtt/mqtt"
	"slices"
	"strings"
	"time"
)

var deviceLookupTable map[string]*homely.Device

func lookupDevice(home *homely.Home, deviceID string) (*homely.Device, error) {
	if deviceLookupTable == nil {
		deviceLookupTable = make(map[string]*homely.Device)
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
	dev = &home.Devices[i]
	deviceLookupTable[deviceID] = dev
	return dev, nil
}

func nameToMqtt(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "_")
}

func main() {
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("unable to create logfile")
	}
	log.SetOutput(file)
	defer file.Close()

	broker := flag.String("broker", "pmx-containers.home.arpa:1883", "Broker URL with port")
	mqttClientID := flag.String("clientID", "homely2mqtt_client", "ID of MQTT client")
	homelyUser := flag.String("homely-user", "", "Homely username")
	homelyPwd := flag.String("homely-password", "", "Homely password")

	flag.Parse()

	log.Printf("creating homely api client")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	homelyClient := homely.NewClient(*homelyUser, *homelyPwd)

	mqttClient := mqtt.NewClient(*broker, *mqttClientID, "/home/homely")
	err = mqttClient.Connect()
	if err != nil {
		log.Fatalf("cant connect to mqtt broker: %s", err.Error())
	}
	defer mqttClient.Disconnect()

	log.Printf("connected to broker")

	err = mqttClient.Publish("test/message", "my message is hello", false)

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
		dev, err := lookupDevice(home, deviceID)
		if err != nil {
			log.Printf("unable to lookup device: %s", err.Error())
			return
		}
		devName := nameToMqtt(dev.Name)

		_ = mqttClient.Publish(fmt.Sprintf("location/%s/%s/%s", nameToMqtt(dev.Location), devName, valueName), value, true)
		_ = mqttClient.Publish(fmt.Sprintf("device/%s/%s", devName, valueName), value, true)
		_ = mqttClient.Publish(fmt.Sprintf("%s/%s", deviceID, valueName), value, true)
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

	err = mqttClient.Publish("homely2mqtt", "connecting to streaming api", false)
	go homelyClient.ConnectHome(ctx, home.LocationID, deviceChanged, alarmChanged)

	for {
		select {
		case <-ctx.Done():
			return
		}
	}
}
