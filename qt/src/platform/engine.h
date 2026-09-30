#pragma once

#include <QByteArray>
#include <QString>
#include <QStringList>
#include <QVariantMap>
#include <cstdint>
#include <functional>

// 共通のエンジン呼び出し。デスクトップはGoのC ABI、wasmはJSへ委譲する。
QVariantMap callNative(uintptr_t handle, const QByteArray &method,
                       const QVariantMap &request = {});

// 初期メタデータを返し、進捗は0..N-1で通知する。
QVariantMap initializeNative(const QByteArray &encoded,
                             const std::function<void(int)> &progress);

void destroyNative(uintptr_t handle);

QString lastNativeError();

// wasmでは外部プロセスを起動せずfalseを返す。
bool startDetachedProcess(const QString &program, const QStringList &arguments,
                          const QString &workingDirectory, qint64 *pid);
