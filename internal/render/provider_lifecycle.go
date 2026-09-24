package render

import (
	"errors"

	"utautts/internal/render/base"
)

// CloseProviderSessionsはアプリ終了時に常駐する外部Providerプロセスを解放する。
func CloseProviderSessions() error {
	// 登録済みの常駐リソース（WORLD bridge clientなど）を解放してから、外部Providerセッションを閉じる。
	return errors.Join(base.CloseRegistered(), externalProviderSessions.closeAll())
}
