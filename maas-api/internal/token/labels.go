package token

func namespaceLabels(instance, tier string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/component":        "token-issuer",
		"app.kubernetes.io/part-of":          "maas-api",
		"maas.opendatahub.io/instance":       instance,
		"maas.opendatahub.io/tier":           tier,
		"maas.opendatahub.io/tier-namespace": "true",
	}
}

func serviceAccountLabels(instance, tier string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/component":  "token-issuer",
		"app.kubernetes.io/part-of":    "maas-api",
		"maas.opendatahub.io/instance": instance,
		"maas.opendatahub.io/tier":     tier,
	}
}
