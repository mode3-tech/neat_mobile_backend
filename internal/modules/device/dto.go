package device

type DeviceBindingRequest struct {
	DeviceID    string `json:"device_id" binding:"required"`
	PublicKey   string `json:"public_key" binding:"required"`
	DeviceName  string `json:"device_name" binding:"required"`
	DeviceModel string `json:"device_model" binding:"required"`
	OS          string `json:"os" binding:"required"`
	OSVersion   string `json:"os_version" binding:"required"`
	AppVersion  string `json:"app_version" binding:"required"`
	IP          string `json:"ip" binding:"required"`
}
