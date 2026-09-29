#include "platform/media.h"

#include <QQmlEngine>
#include <QtQml>

void registerMediaTypes() {
    qmlRegisterType<MediaPlayer>("UtauTTS.Media", 1, 0, "MediaPlayer");
    qmlRegisterType<MediaOutput>("UtauTTS.Media", 1, 0, "AudioOutput");
    qmlRegisterType<MediaDevices>("UtauTTS.Media", 1, 0, "MediaDevices");
}
