package constant

import "time"

const (
	TierMappingConfigMap    = "tier-to-group-mapping"
	DefaultNamespace        = "maas-api"
	DefaultGatewayName      = "maas-default-gateway"
	DefaultGatewayNamespace = "openshift-ingress"

	DefaultResyncPeriod = 8 * time.Hour
)
