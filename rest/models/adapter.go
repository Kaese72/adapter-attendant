package models

import (
	"time"
)

type Adapter struct {
	ID        int        `json:"id" readOnly:"true"`
	Name      string     `json:"name" maxLength:"255"`
	ImageName string     `json:"imageName" maxLength:"255"`
	ImageTag  string     `json:"imageTag" maxLength:"64"`
	Created   time.Time  `json:"created" readOnly:"true"`
	Updated   time.Time  `json:"updated" readOnly:"true"`
	Synced    *time.Time `json:"synced,omitempty" readOnly:"true"`
	// Address    string     `json:"address"`
	// AdapterKey string     `json:"adapterKey"`
}

type AdapterConfiguration struct {
	ID          int       `json:"id" readOnly:"true"`
	AdapterID   int       `json:"adapterId" readOnly:"true"`
	ConfigKey   string    `json:"configKey" maxLength:"255"`
	ConfigValue string    `json:"configValue" maxLength:"4096"`
	Created     time.Time `json:"created" readOnly:"true"`
	Updated     time.Time `json:"updated" readOnly:"true"`
}
