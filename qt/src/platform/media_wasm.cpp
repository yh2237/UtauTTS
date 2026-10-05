#include "platform/media.h"

#include <QString>
#include <QTimer>

#ifdef __EMSCRIPTEN__
#include <emscripten/val.h>
#endif


struct MediaOutput::Impl {
    qreal volume = 1.0;
    bool muted = false;
    QVariant device;
};

MediaOutput::MediaOutput(QObject *parent) : QObject(parent), m_impl(std::make_unique<Impl>()) {}
MediaOutput::~MediaOutput() = default;
qreal MediaOutput::volume() const { return m_impl->volume; }
void MediaOutput::setVolume(qreal value) {
    if (m_impl->volume == value) {
        return;
    }
    m_impl->volume = value;
    emit volumeChanged();
}
bool MediaOutput::muted() const { return m_impl->muted; }
void MediaOutput::setMuted(bool value) {
    if (m_impl->muted == value) {
        return;
    }
    m_impl->muted = value;
    emit mutedChanged();
}
QVariant MediaOutput::device() const { return m_impl->device; }
void MediaOutput::setDevice(const QVariant &value) {
    m_impl->device = value;
    emit deviceChanged();
}
void *MediaOutput::nativeHandle() const { return nullptr; }

struct MediaDevices::Impl {
    QVariantList outputs;
    QVariant defaultOutput;
};

MediaDevices::MediaDevices(QObject *parent) : QObject(parent), m_impl(std::make_unique<Impl>()) {}
MediaDevices::~MediaDevices() = default;
QVariantList MediaDevices::audioOutputs() const { return m_impl->outputs; }
QVariant MediaDevices::defaultAudioOutput() const { return m_impl->defaultOutput; }

namespace {
qreal outputVolume(const QObject *output) {
    if (auto *media = qobject_cast<const MediaOutput *>(output)) {
        return media->volume();
    }
    return 1.0;
}
bool outputMuted(const QObject *output) {
    if (auto *media = qobject_cast<const MediaOutput *>(output)) {
        return media->muted();
    }
    return false;
}
void callJs(const char *name) {
#ifdef __EMSCRIPTEN__
    emscripten::val function = emscripten::val::global(name);
    if (!function.isUndefined()) {
        function();
    }
#endif
}

#ifdef __EMSCRIPTEN__
qreal callJsNumber(const char *name) {
    emscripten::val function = emscripten::val::global(name);
    return function.isUndefined() ? 0 : function().as<double>();
}
bool callJsBool(const char *name) {
    emscripten::val function = emscripten::val::global(name);
    return function.isUndefined() ? false : function().as<bool>();
}
#else
qreal callJsNumber(const char *) { return 0; }
bool callJsBool(const char *) { return false; }
#endif
}

struct MediaPlayer::Impl {
    QUrl source;
    QObject *audioOutputObject = nullptr;
    int playbackState = MediaPlayer::StoppedState;
    int mediaStatus = MediaPlayer::NoMedia;
    qreal position = 0;
    qreal duration = 0;
    QString error;
    QTimer *timer = nullptr;
};

MediaPlayer::MediaPlayer(QObject *parent) : QObject(parent), m_impl(std::make_unique<Impl>()) {
    m_impl->timer = new QTimer(this);
    m_impl->timer->setInterval(80);
    connect(m_impl->timer, &QTimer::timeout, this, [this]() {
        if (m_impl->playbackState != PlayingState) {
            return;
        }
        const qreal position = callJsNumber("utauttsMediaPosition");
        const qreal duration = callJsNumber("utauttsMediaDuration");
        if (position != m_impl->position) {
            m_impl->position = position;
            emit positionChanged();
        }
        if (duration != m_impl->duration) {
            m_impl->duration = duration;
            emit durationChanged();
        }
        if (callJsBool("utauttsMediaEnded")) {
            m_impl->timer->stop();
            m_impl->playbackState = StoppedState;
            m_impl->mediaStatus = EndOfMedia;
            emit playbackStateChanged();
            emit mediaStatusChanged();
        }
    });
}
MediaPlayer::~MediaPlayer() = default;
QUrl MediaPlayer::source() const { return m_impl->source; }
void MediaPlayer::setSource(const QUrl &value) {
    if (m_impl->source == value) {
        return;
    }
    m_impl->source = value;
    m_impl->mediaStatus = value.isEmpty() ? NoMedia : LoadedMedia;
    emit sourceChanged();
    emit mediaStatusChanged();
}
QObject *MediaPlayer::audioOutput() const { return m_impl->audioOutputObject; }
void MediaPlayer::setAudioOutput(QObject *value) {
    m_impl->audioOutputObject = value;
    emit audioOutputChanged();
}
int MediaPlayer::playbackState() const { return m_impl->playbackState; }
int MediaPlayer::mediaStatus() const { return m_impl->mediaStatus; }
QString MediaPlayer::errorString() const { return m_impl->error; }
qreal MediaPlayer::position() const { return m_impl->position; }
void MediaPlayer::setPosition(qreal value) {
    m_impl->position = value;
    emit positionChanged();
#ifdef __EMSCRIPTEN__
    emscripten::val function = emscripten::val::global("utauttsMediaSeek");
    if (!function.isUndefined()) {
        function(static_cast<double>(value));
    }
#endif
}
qreal MediaPlayer::duration() const { return m_impl->duration; }
void MediaPlayer::play() {
#ifdef __EMSCRIPTEN__
    emscripten::val function = emscripten::val::global("utauttsMediaPlay");
    if (!function.isUndefined() && !m_impl->source.isEmpty()) {
        function(m_impl->source.toString().toStdString(), outputVolume(m_impl->audioOutputObject),
                 outputMuted(m_impl->audioOutputObject));
    }
#endif
    m_impl->playbackState = PlayingState;
    m_impl->mediaStatus = BufferedMedia;
    m_impl->position = 0;
    if (m_impl->timer) {
        m_impl->timer->start();
    }
    emit playbackStateChanged();
    emit mediaStatusChanged();
    emit positionChanged();
}
void MediaPlayer::pause() {
    callJs("utauttsMediaPause");
    if (m_impl->timer) {
        m_impl->timer->stop();
    }
    m_impl->playbackState = PausedState;
    emit playbackStateChanged();
}
void MediaPlayer::stop() {
    callJs("utauttsMediaStop");
    if (m_impl->timer) {
        m_impl->timer->stop();
    }
    m_impl->playbackState = StoppedState;
    m_impl->mediaStatus = m_impl->source.isEmpty() ? NoMedia : LoadedMedia;
    emit playbackStateChanged();
    emit mediaStatusChanged();
}
