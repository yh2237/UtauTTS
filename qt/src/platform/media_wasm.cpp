#include "platform/media.h"

#include <QString>

#ifdef __EMSCRIPTEN__
#include <emscripten/val.h>
#endif

// wasm 実装: QML の API を満たしつつ、実際の再生は Web Audio（JS ヘルパ）へ委譲する。

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
}  // namespace

struct MediaPlayer::Impl {
    QUrl source;
    QObject *audioOutputObject = nullptr;
    int playbackState = MediaPlayer::StoppedState;
    int mediaStatus = MediaPlayer::NoMedia;
    QString error;
};

MediaPlayer::MediaPlayer(QObject *parent) : QObject(parent), m_impl(std::make_unique<Impl>()) {}
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
    emit playbackStateChanged();
    emit mediaStatusChanged();
}
void MediaPlayer::pause() {
    callJs("utauttsMediaPause");
    m_impl->playbackState = PausedState;
    emit playbackStateChanged();
}
void MediaPlayer::stop() {
    callJs("utauttsMediaStop");
    m_impl->playbackState = StoppedState;
    m_impl->mediaStatus = m_impl->source.isEmpty() ? NoMedia : LoadedMedia;
    emit playbackStateChanged();
    emit mediaStatusChanged();
}
