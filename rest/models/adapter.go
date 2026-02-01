package models

import (
	"time"
)

type Adapter struct {
	ID        int        `json:"id" readOnly:"true"`
	Name      string     `json:"name"`
	ImageName string     `json:"imageName"`
	ImageTag  string     `json:"imageTag"`
	Created   time.Time  `json:"created" readOnly:"true"`
	Updated   time.Time  `json:"updated" readOnly:"true"`
	Synced    *time.Time `json:"synced,omitempty" readOnly:"true"`
	// Address    string     `json:"address"`
	// AdapterKey string     `json:"adapterKey"`
}

type AdapterConfiguration struct {
	ID          int       `json:"id" readOnly:"true"`
	AdapterID   int       `json:"adapterId" readOnly:"true"`
	ConfigKey   string    `json:"configKey"`
	ConfigValue string    `json:"configValue"`
	Created     time.Time `json:"created" readOnly:"true"`
	Updated     time.Time `json:"updated" readOnly:"true"`
}
