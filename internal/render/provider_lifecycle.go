package render

import (
	"errors"

	"utautts/internal/render/base"
)

// CloseProviderSessionsはアプリ終了時に常駐する外部Providerプロセスを解放する。
func CloseProviderSessions() error {
	return errors.Join(base.CloseRegistered(), externalProviderSessions.closeAll())
}
