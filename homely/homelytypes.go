package homely

import "time"

type Location struct {
	Name          string
	Role          string
	UserId        string // use uuid.UUID?
	LocationID    string // use uuid.UUID?
	GatewaySerial string
}

type Home struct {
	Name               string
	LocationID         string
	Gatewayserial      string
	UserRoleAtLocation string
	AlarmState         string
	Devices            []Device
}

type Device struct {
	ID           string
	Name         string
	SerialNumber string
	Location     string
	Online       bool
	ModelID      string
	ModelName    string
	Features     Features
}

type Features struct {
	Setup       *Setup       `json:"setup,omitempty"`
	Alarm       *Alarm       `json:"alarm,omitempty"`
	Temperature *Temperature `json:"temperature,omitempty"`
	Battery     *Battery     `json:"battery,omitempty"`
	Diagnostic  *Diagnostic  `json:"diagnostic,omitempty"`
}

type StateBool struct {
	Value       bool
	LastUpdated time.Time
}

type StateInt struct {
	Value       int
	LastUpdated time.Time
}

type StateString struct {
	Value       string
	LastUpdated time.Time
}

type StateFloat struct {
	Value       float32
	LastUpdated time.Time
}

type Setup struct {
	States SetupStates
}

type SetupStates struct {
	AppLedEnable StateBool
	ErrLedEnable StateBool
}

type Alarm struct {
	States AlarmStates
}

type AlarmStates struct {
	Alarm          StateBool
	Tamper         StateBool
	SensitivyLevel StateInt
}

type Temperature struct {
	States TemperatureStates
}

type TemperatureStates struct {
	Temperature StateFloat
}

type BatteryStates struct {
	Low     StateBool
	Defect  StateBool
	Voltage StateFloat
}
type Battery struct {
	States BatteryStates
}

type DiagnosticStates struct {
	NetworkLinkStrength StateInt
	NetworkLinkAddress  StateString
}
type Diagnostic struct {
	States DiagnosticStates
}

// type Change struct {
// 	Feature    string
// 	LastUpdate *time.Time
// 	StateName  string
// 	Value      any
// }

type DeviceChangeData struct {
	StateName string
	Value     any
}

type EventDeviceStateChanged struct {
	DeviceID string
	Changes  []DeviceChangeData
}

type EventAlarmStateChanged struct {
	DeviceID  string
	UserName  string
	State     string
	Timestamp time.Time
}
