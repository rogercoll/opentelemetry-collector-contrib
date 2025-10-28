package profilingsessionsobserver

type Session struct {
	// Config holds configuration settings that should be sent to
	// agents matching the above constraints.
	Resources map[string]string
	// ServiceName holds the service name to which this agent configuration
	// applies. This is optional.
	Duration string
}
