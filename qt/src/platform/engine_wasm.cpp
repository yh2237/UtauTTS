#include "platform/engine.h"

#include <QJsonDocument>
#include <QJsonObject>
#include <QJsonParseError>
#include <QString>

#ifdef __EMSCRIPTEN__
#include <emscripten/val.h>
#endif

// 同期モード用。Qtのイベントループを中断しないよう、失敗は_errorで返す。

QVariantMap callNative(uintptr_t, const QByteArray &method, const QVariantMap &request) {
#ifdef __EMSCRIPTEN__
    emscripten::val api = emscripten::val::global("utauttsWasm");
    if (api.isUndefined()) {
        return {{QStringLiteral("_error"), QStringLiteral("wasm engine (utauttsWasm) is not loaded")}};
    }
    const QByteArray requestJSON =
        QJsonDocument::fromVariant(request).toJson(QJsonDocument::Compact);
    emscripten::val response = api.call<emscripten::val>(
        "call", std::string(method.constData()), std::string(requestJSON.constData()));
    const QString text = QString::fromStdString(response.as<std::string>());
    QJsonParseError parseError;
    const QJsonDocument document = QJsonDocument::fromJson(text.toUtf8(), &parseError);
    if (parseError.error != QJsonParseError::NoError || !document.isObject()) {
        return {{QStringLiteral("_error"), QStringLiteral("wasm engine returned invalid JSON")}};
    }
    const QJsonObject object = document.object();
    if (!object.value(QStringLiteral("ok")).toBool()) {
        return {{QStringLiteral("_error"), object.value(QStringLiteral("error")).toString()}};
    }
    return object.value(QStringLiteral("result")).toObject().toVariantMap();
#else
    (void)method;
    (void)request;
    return {{QStringLiteral("_error"), QStringLiteral("wasm engine is unavailable in this build")}};
#endif
}

QVariantMap initializeNative(const QByteArray &, const std::function<void(int)> &progress) {
    if (progress) {
        progress(0);
    }
    const QVariantMap voices = callNative(0, "voicebanks");
    if (voices.contains(QStringLiteral("_error"))) {
        return voices;
    }
    if (progress) {
        progress(1);
    }
    const QVariantMap models = callNative(0, "models");
    if (progress) {
        progress(2);
    }
    const QVariantMap renderers = callNative(0, "renderers");
    if (progress) {
        progress(3);
        progress(4);
    }
    return {
        {"_handle", QVariant::fromValue<qulonglong>(1)},
        {"voicebanks", voices.value("voicebanks")},
        {"models", models.value("models")},
        {"renderers", renderers.value("renderers")},
        {"problems", renderers.value("problems")},
        {"resamplers", renderers.value("resamplers")},
        {"wavtools", renderers.value("wavtools")},
        {"default_renderer", renderers.value("default_renderer")},
    };
}

void destroyNative(uintptr_t) {}

QString lastNativeError() { return QString(); }

bool startDetachedProcess(const QString &, const QStringList &, const QString &, qint64 *) {
    return false;
}
