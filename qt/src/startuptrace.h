#pragma once

#include <QByteArray>
#include <QDebug>
#include <QElapsedTimer>
#include <QtGlobal>

// UTAUTTS_STARTUP_TRACE=1 のとき、起動の各段階までの経過時間を標準エラーへ出す。
inline QElapsedTimer &startupTraceClock() {
    static QElapsedTimer clock = [] {
        QElapsedTimer timer;
        timer.start();
        return timer;
    }();
    return clock;
}

inline void startupTrace(const char *stage) {
    static const bool enabled = qEnvironmentVariableIntValue("UTAUTTS_STARTUP_TRACE") != 0;
    if (enabled)
        qInfo().noquote() << "startup-trace" << startupTraceClock().elapsed() << "ms" << stage;
}
