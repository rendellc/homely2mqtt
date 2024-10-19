package homely

import (
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
)

func mapGet[V any](m map[string]any, keys ...string) (V, bool) {
	var invalidValue V
	var current any = m
	for i := range len(keys) - 1 {
		key := keys[i]

		currentMap, currentIsMap := current.(map[string]any)
		if !currentIsMap {
			// keys do not match the structure of the map
			log.Printf("expected map, found: %v", current)
			return invalidValue, false
		}

		valueAny, keyPresent := currentMap[key]
		if !keyPresent {
			log.Printf("expected key %s in %v", key, currentMap)
			return invalidValue, false
		}
		current = valueAny
	}

	lastMap, lastIsMap := current.(map[string]any)
	if !lastIsMap {
		log.Printf("expected last to be a map, found %v", current)
		return invalidValue, false
	}

	finalKey := keys[len(keys)-1]
	valueAny, found := lastMap[finalKey]
	if !found {
		log.Printf("cant find key %s in %v", finalKey, lastMap)
		return invalidValue, false
	}

	value, valueCorrectType := valueAny.(V)
	if !valueCorrectType {
		log.Printf("mismatch type for value, cant coerce %v to be of type %v. Found %v", valueAny, reflect.TypeOf(invalidValue), reflect.TypeOf(valueAny))
		return invalidValue, false
	}

	return value, true
}

func parseDeviceStateChangeEvent(eventAny any) (EventDeviceStateChanged, error) {
	var invalid EventDeviceStateChanged
	event, ok := eventAny.(map[string]any)
	if !ok {
		return invalid, fmt.Errorf("unexpected event type: %v, %v", reflect.TypeOf(eventAny), eventAny)
	}

	deviceID, ok := mapGet[string](event, "data", "deviceId")
	if !ok {
		return invalid, fmt.Errorf("unable to find device id: %v", event)
	}
	changes, ok := mapGet[[]any](event, "data", "changes")
	if !ok {
		return invalid, fmt.Errorf("unable to find changes: %v", event)
	}

	deviceChanges := []DeviceChangeData{}
	for _, change := range changes {
		changeMap, ok := change.(map[string]any)
		if !ok {
			return invalid, fmt.Errorf("unable to interpret change as map, found %v of type %v", change, reflect.TypeOf(change))
		}

		statename, ok := mapGet[string](changeMap, "stateName")
		if !ok {
			return invalid, fmt.Errorf("unable to find stateName in %v", change)
		}
		value, ok := mapGet[any](changeMap, "value")
		if !ok {
			return invalid, fmt.Errorf("unable to find value in %v", change)
		}

		deviceChanges = append(deviceChanges, DeviceChangeData{
			StateName: statename,
			Value:     value,
		})
	}

	return EventDeviceStateChanged{
		DeviceID: deviceID,
		Changes:  deviceChanges,
	}, nil
}

// 2024/10/10 07:43:10 event-type: alarm-state-changed {"Data":{"Changes":null,"DeviceID":"","GatewayID":"","LocationID":"a34c83af-3ebe-433d-9f71-c6718c0fb647","ModelID":"","RootLocationID":""},"Type":"alarm-state-changed"}
// 2024/10/10 16:42:03 event: {"data":{"locationId":"a34c83af-3ebe-433d-9f71-c6718c0fb647","partnerCode":1275,"state":"ARM_PENDING","timestamp":"2024-10-10T14:42:03.290Z"},"type":"alarm-state-changed"}
// 2024/10/10 16:42:06 event: {"data":{"deviceId":"f3660d94-14cb-4909-b61f-04912a340917","eventId":594,"locationId":"a34c83af-3ebe-433d-9f71-c6718c0fb647","partnerCode":1275,"state":"DISARMED","timestamp":"2024-10-10T14:42:06.300Z","userId":"452e7b85-e94d-4742-8ff0-670a056ad908","userName":"Rendell Cale"},"type":"alarm-state-changed"}
// 2024/10/10 07:43:10 event-type: alarm-state-changed
// {"Data":{"Changes":null,"DeviceID":"","GatewayID":"","LocationID":"a34c83af-3ebe-433d-9f71-c6718c0fb647","ModelID":"","RootLocationID":""},"Type":"alarm-state-changed"}
type alarmChangeData struct {
	DeviceID    string    `json:"deviceId"`
	EventID     string    `json:"eventId"`
	LocationID  string    `json:"locationId"`
	PartnerCode string    `json:"partnerCode"`
	State       string    `json:"state"`
	Timestamp   time.Time `json:"timestamp"`
	UserID      string    `json:"userId"`
	UserName    string    `json:"userName"`
}

func parseAlarmStateChangeEvent(eventAny any) (EventAlarmStateChanged, error) {
	var invalid EventAlarmStateChanged
	type alarmEvent struct {
		Data alarmChangeData
	}
	event := alarmEvent{}
	err := mapstructure.Decode(eventAny, &event)
	if err != nil {
		return invalid, fmt.Errorf("unable to decode %v as alarmEvent: %w", eventAny, err)
	}

	return EventAlarmStateChanged{
		DeviceID:  event.Data.DeviceID,
		UserName:  event.Data.UserName,
		State:     event.Data.State,
		Timestamp: event.Data.Timestamp,
	}, nil
}
