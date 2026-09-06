package render

// CloseProviderSessions releases long-lived external Provider processes. It
// is intended for application shutdown; normal synthesis keeps sessions
// resident so model and native runtime initialization is not repeated.
func CloseProviderSessions() error {
	// Serialize with the WORLD bridge client before replacing its shared
	// session. Session.Close itself waits for an in-flight render to finish or
	// reach its cancellation/termination grace period.
	worldlineBridgeGate <- struct{}{}
	sharedWorldlineBridge.stop()
	<-worldlineBridgeGate
	return externalProviderSessions.closeAll()
}
