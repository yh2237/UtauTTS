#include "platform/engine.h"

#include "utautts_abi.h"

#include <QJsonDocument>
#include <QProcess>
#include <QJsonObject>
#include <QJsonParseError>
#include <QJsonValue>
#include <QString>
#include <memory>
#include <stdexcept>

QVariantMap callNative(uintptr_t handle, const QByteArray &method,
                       const QVariantMap &request) {
    if (!handle) {
        throw std::runtime_error("native backend is not initialized");
    }
    QByteArray methodCopy = method;
    QByteArray requestJSON = QJsonDocument::fromVariant(request).toJson(QJsonDocument::Compact);
    std::unique_ptr<char, decltype(&UtauTTSFree)> response(
        UtauTTSCall(handle, methodCopy.data(), requestJSON.data()), &UtauTTSFree);
    if (!response) {
        throw std::runtime_error("native backend returned no response");
    }
    QJsonParseError parseError;
    const QJsonDocument document = QJsonDocument::fromJson(response.get(), &parseError);
    if (parseError.error != QJsonParseError::NoError || !document.isObject()) {
        throw std::runtime_error("native backend returned invalid JSON");
    }
    const QJsonObject object = document.object();
    if (!object.value("ok").toBool()) {
        throw std::runtime_error(object.value("error").toString().toStdString());
    }
    const QJsonValue result = object.value("result");
    if (!result.isObject()) {
        throw std::runtime_error("native backend returned no result");
    }
    return result.toObject().toVariantMap();
}

QVariantMap initializeNative(const QByteArray &encoded,
                             const std::function<void(int)> &progress) {
    QByteArray config = encoded;
    const uintptr_t handle = UtauTTSCreate(config.data());
    if (!handle) {
        std::unique_ptr<char, decltype(&UtauTTSFree)> detail(
            UtauTTSLastError(), &UtauTTSFree);
        const QString message = detail ? QString::fromUtf8(detail.get()) : QString();
        return {{"_error", message.isEmpty()
                              ? QStringLiteral("could not initialize the native backend")
                              : message}};
    }
    try {
        if (progress) {
            progress(0);
        }
        const QVariantMap voices = callNative(handle, "voicebanks");
        if (progress) {
            progress(1);
        }
        const QVariantMap models = callNative(handle, "models");
        if (progress) {
            progress(2);
        }
        const QVariantMap renderers = callNative(handle, "renderers");
        if (progress) {
            progress(3);
            progress(4);
        }
        return {
            {"_handle", QVariant::fromValue<qulonglong>(static_cast<qulonglong>(handle))},
            {"voicebanks", voices.value("voicebanks")},
            {"models", models.value("models")},
            {"renderers", renderers.value("renderers")},
            {"problems", renderers.value("problems")},
            {"resamplers", renderers.value("resamplers")},
            {"wavtools", renderers.value("wavtools")},
            {"default_renderer", renderers.value("default_renderer")},
        };
    } catch (const std::exception &exception) {
        UtauTTSDestroy(handle);
        return {{"_error", QString::fromUtf8(exception.what())}};
    }
}

void destroyNative(uintptr_t handle) {
    if (handle) {
        UtauTTSDestroy(handle);
    }
}

QString lastNativeError() {
    std::unique_ptr<char, decltype(&UtauTTSFree)> detail(UtauTTSLastError(), &UtauTTSFree);
    return detail ? QString::fromUtf8(detail.get()) : QString();
}
bool startDetachedProcess(const QString &program, const QStringList &arguments,
                          const QString &workingDirectory, qint64 *pid) {
    return QProcess::startDetached(program, arguments, workingDirectory, pid);
}