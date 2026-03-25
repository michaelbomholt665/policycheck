package router

// CapabilityManifestEntry describes one router-native capability family.
type CapabilityManifestEntry struct {
	Port     PortName
	Resolver string
}

var declaredCapabilities = []CapabilityManifestEntry{
	{
		Port:     PortCLIStyle,
		Resolver: "capabilities.ResolveCLIOutputStyler",
	},
}

// RouterDeclaredCapabilities returns the declared router-native capability manifest.
func RouterDeclaredCapabilities() []CapabilityManifestEntry {
	manifest := make([]CapabilityManifestEntry, len(declaredCapabilities))
	copy(manifest, declaredCapabilities)

	return manifest
}
