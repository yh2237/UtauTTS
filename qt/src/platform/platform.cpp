#include "platform/platform.h"

#include <QtQml>

#ifdef UTAUTTS_WASM
#include <emscripten/val.h>
#endif

Platform::Platform(QObject *parent)
    : QObject(parent) {
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
#ifdef UTAUTTS_WASM
    // スマホ/タブレット判定はタッチ対応かつ小さいビューポートを目安にする。
    // 将来 MobileShell を作る際にここを調整する。
    const emscripten::val navigator = emscripten::val::global("navigator");
    const int touchPoints = navigator["maxTouchPoints"].isUndefined()
            ? 0 : navigator["maxTouchPoints"].as<int>();
    const emscripten::val window = emscripten::val::global("window");
    const double width = window["innerWidth"].isUndefined()
            ? 0.0 : window["innerWidth"].as<double>();
    return touchPoints > 0 && width > 0.0 && width < 768.0;
#else
    return false;
#endif
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
