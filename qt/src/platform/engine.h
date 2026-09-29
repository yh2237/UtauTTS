#pragma once

#include <QByteArray>
#include <QString>
#include <QStringList>
#include <QVariantMap>
#include <cstdint>
#include <functional>

// エンジン呼び出しのプラットフォーム抽象。
// desktop は Go の C ABI を同期で呼ぶ。wasm は実装側で JS ブリッジへ委譲する。
QVariantMap callNative(uintptr_t handle, const QByteArray &method,
                       const QVariantMap &request = {});

// initializeNative は config からエンジンを起動し、初期メタデータを返す。
// 進捗は 0..N-1 の範囲で progress に通知する。
QVariantMap initializeNative(const QByteArray &encoded,
                             const std::function<void(int)> &progress);

// destroyNative はエンジンを解放する。
void destroyNative(uintptr_t handle);

// lastNativeError は直近のエラー文言を返す。
QString lastNativeError();

// startDetachedProcess はプロセスを切り離して起動する（デスクトップ専用）。
// wasm では常に false を返す。
bool startDetachedProcess(const QString &program, const QStringList &arguments,
                          const QString &workingDirectory, qint64 *pid);


