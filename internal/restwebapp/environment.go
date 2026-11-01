package restwebapp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Kaese72/adapter-attendant/internal/config"
	"github.com/Kaese72/adapter-attendant/internal/utility"
	"github.com/pkg/errors"
)

// adapterResourceName is the name of every Kubernetes object ArgoCD deploys
// for an adapter: its Deployment, Service and Secret.
func adapterResourceName(adapterId int) string {
	return fmt.Sprintf("adapter-%d", adapterId)
}

// adapterEnvironment is the whole environment an adapter runs with, as
// recorded at sync time and handed to ArgoCD by the plugin endpoint.
func adapterEnvironment(adapterId int, userProvidedConfiguration map[string]string) (map[string]string, error) {
	jwtToken, err := utility.GenerateAdapterJWT(config.Loaded.Adapters.DeviceStoreJWTSecret, 24*30*12*time.Hour, adapterId)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate enrollment token")
	}
	// Add mandatory configuration that is not visible to user
	// System provided configuration is namespace with "HUEMIE_".
	environment := map[string]string{
		"HUEMIE_ENROLL_TOKEN": jwtToken,
		"HUEMIE_ENROLL_STORE": config.Loaded.Adapters.DeviceStoreURL,
	}
	// User provided configuration needs to be namespaced with "ADAPTER_"
	// To prevent collision with system provided configuration
	for k, v := range userProvidedConfiguration {
		environment[fmt.Sprintf("ADAPTER_%s", k)] = v
	}
	return environment, nil
}

// secretData turns a recorded environment (syncedEnvironment) into the data
// of the adapter's Secret: the same keys, base64-encoded values. data rather
// than stringData, since keys dropped from stringData are never removed from
// the live Secret.
func secretData(syncedEnvironment string) (map[string]string, error) {
	environment := map[string]string{}
	if err := json.Unmarshal([]byte(syncedEnvironment), &environment); err != nil {
		return nil, errors.Wrap(err, "failed to parse synced environment")
	}
	data := make(map[string]string, len(environment))
	for k, v := range environment {
		data[k] = base64.StdEncoding.EncodeToString([]byte(v))
	}
	return data, nil
}
