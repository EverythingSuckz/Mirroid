package model

// DeviceStatus represents the display status of a device in the UI.
type DeviceStatus string

const (
	StatusConnected    DeviceStatus = "Connected"
	StatusLaunching    DeviceStatus = "Launching"
	StatusMirroring    DeviceStatus = "Mirroring"
	StatusError        DeviceStatus = "Error"
	StatusDisconnected DeviceStatus = "Disconnected"
	StatusReconnecting DeviceStatus = "Reconnecting"
)
