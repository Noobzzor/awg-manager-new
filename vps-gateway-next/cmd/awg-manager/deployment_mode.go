package main

import "fmt"

type deploymentMode string

const (
	deploymentModeKeenetic deploymentMode = "keenetic"
	deploymentModeSingbox  deploymentMode = "singbox"
)

// parseDeploymentMode distinguishes an unset variable from an explicitly
// empty one. Callers must use an os.LookupEnv-shaped function.
func parseDeploymentMode(lookupEnv func(string) (string, bool)) (deploymentMode, error) {
	value, set := lookupEnv("AWG_MODE")
	if !set {
		return deploymentModeKeenetic, nil
	}
	return parseDeploymentModeValue(value)
}

func parseDeploymentModeValue(value string) (deploymentMode, error) {
	switch deploymentMode(value) {
	case deploymentModeKeenetic:
		return deploymentModeKeenetic, nil
	case deploymentModeSingbox:
		return deploymentModeSingbox, nil
	default:
		return "", fmt.Errorf("invalid AWG_MODE %q: expected keenetic or singbox", value)
	}
}
