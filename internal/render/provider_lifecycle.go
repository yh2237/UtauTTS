package render

// CloseProviderSessionsはアプリ終了時に常駐する外部Providerプロセスを解放する。
func CloseProviderSessions() error {
	// WORLD bridge clientと直列化してから共有セッションを置き換える。Close自体は実行中renderの完了を待つ。
	worldlineBridgeGate <- struct{}{}
	sharedWorldlineBridge.stop()
	<-worldlineBridgeGate
	return externalProviderSessions.closeAll()
}
