package restwebapp

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/Kaese72/adapter-attendant/internal/config"
)

func TestAdapterEnvironment(t *testing.T) {
	config.Loaded.Adapters.DeviceStoreJWTSecret = "test-secret"
	config.Loaded.Adapters.DeviceStoreURL = "http://device-store:8080"
	environment, err := adapterEnvironment(7, map[string]string{"HUE_API_KEY": "key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if environment["ADAPTER_HUE_API_KEY"] != "key" {
		t.Errorf("user argument not prefixed: %v", environment)
	}
	if environment["HUEMIE_ENROLL_STORE"] != "http://device-store:8080" || environment["HUEMIE_ENROLL_TOKEN"] == "" {
		t.Errorf("system configuration missing: %v", environment)
	}
}

func TestSecretData(t *testing.T) {
	encoded, _ := json.Marshal(map[string]string{"ADAPTER_URL": "http://192.168.1.2/", "QUOTE": `"\`})
	data, err := secretData(string(encoded))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for key, want := range map[string]string{"ADAPTER_URL": "http://192.168.1.2/", "QUOTE": `"\`} {
		got, err := base64.StdEncoding.DecodeString(data[key])
		if err != nil || string(got) != want {
			t.Errorf("%s: got %q (%v), want %q", key, got, err, want)
		}
	}
	if _, err := secretData("not json"); err == nil {
		t.Error("expected an error for a malformed environment")
	}
}
