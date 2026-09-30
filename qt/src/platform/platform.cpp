#include "platform/platform.h"

#include <QtQml>
#include <QUrlQuery>

#ifdef UTAUTTS_WASM
#include <emscripten/val.h>
#endif

Platform::Platform(QObject *parent)
    : QObject(parent) {
#ifdef UTAUTTS_WASM
    const auto window = emscripten::val::global("window");
    const QString search = QString::fromStdString(window["location"]["search"].as<std::string>());
    const QString override = QUrlQuery(search.startsWith('?') ? search.mid(1) : search)
            .queryItemValue(QStringLiteral("mobile"));
    if (override == QLatin1String("1"))
        m_mobileOverride = 1;
    else if (override == QLatin1String("0"))
        m_mobileOverride = 0;
    if (!window["innerWidth"].isUndefined())
        updateViewportWidth(window["innerWidth"].as<double>());
#endif
}

bool Platform::isWeb() const {
#ifdef UTAUTTS_WASM
    return true;
#else
    return false;
#endif
}

bool Platform::isDesktop() const {
    return !isWeb();
}

bool Platform::isMobile() const {
    return m_mobile;
}

void Platform::updateViewportWidth(qreal width) {
#ifdef UTAUTTS_WASM
    // Use the browser viewport rather than a transient QML initial size or
    // the client area reduced by Qt's window decorations.
    const auto viewport = emscripten::val::global("window")["innerWidth"];
    if (!viewport.isUndefined())
        width = viewport.as<double>();
#endif
    const bool mobile = isWeb() && (m_mobileOverride >= 0
            ? m_mobileOverride == 1 : width > 0 && width < 768);
#ifdef UTAUTTS_WASM
    // Expose the active layout to the HTML host as well as QML.
    const auto screen = emscripten::val::global("document")
            .call<emscripten::val>("getElementById", std::string("screen"));
    if (!screen.isNull() && !screen.isUndefined())
        screen.call<void>("setAttribute", std::string("data-layout"),
                          std::string(mobile ? "mobile" : "desktop"));
#endif
    if (mobile == m_mobile)
        return;
    m_mobile = mobile;
    emit layoutChanged();
}

bool Platform::hasNativeFileDialog() const {
    // wasm はネイティブのファイルダイアログを持たない。保存はブラウザダウンロード、
    // 読み込みはファイル選択（<input type=file>）で代替する。
    return isDesktop();
}

bool Platform::hasExternalTools() const {
    // 外部プロセス（classic UTAUのresampler/wavtool、ffmpeg等）は
    // ネイティブ環境のみ。wasmでは無効。
    return isDesktop();
}

bool Platform::hasDiffsinger() const {
    // DiffSingerブリッジはネイティブ(Windows)専用。
    return isDesktop();
}

QString Platform::name() const {
    if (!isWeb())
        return QStringLiteral("desktop");
    return isMobile() ? QStringLiteral("web-mobile") : QStringLiteral("web");
}

void registerPlatformSingleton() {
    static Platform *instance = new Platform();
    qmlRegisterSingletonInstance("UtauTTS.Platform", 1, 0, "Platform", instance);
}
