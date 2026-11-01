package models

// ArgoCDPluginGetParamsRequest is the body ArgoCD's ApplicationSet Plugin
// generator sends to POST .../api/v1/getparams.execute. Its contents are
// ignored, but both fields must exist so huma's generated schema, which
// defaults to additionalProperties: false, doesn't reject the body with a 422.
type ArgoCDPluginGetParamsRequest struct {
	ApplicationSetName string                 `json:"applicationSetName,omitempty"`
	Input              map[string]interface{} `json:"input,omitempty"`
}

// ArgoCDPluginParameter is one entry of output.parameters in a Plugin
// generator response, one per synced adapter. Field names become template
// variables ({{.id}}, {{.image}}, {{.syncGeneration}}, {{.secretData}}) in
// huemie-gitops-base's appliance/argocd/applicationset-adapters.yaml. All
// strings, so the Go template never renders a number in exponent notation.
type ArgoCDPluginParameter struct {
	ID             string `json:"id"`
	Image          string `json:"image"`
	SyncGeneration string `json:"syncGeneration"`
	// SecretData is the data of the adapter's Secret, which its Deployment
	// takes its whole environment from: values base64-encoded, credentials
	// included.
	SecretData map[string]string `json:"secretData"`
}

type ArgoCDPluginGetParamsResponse struct {
	Output struct {
		Parameters []ArgoCDPluginParameter `json:"parameters"`
	} `json:"output"`
}
