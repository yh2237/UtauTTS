#include "platform/media.h"

#include <QAudioDevice>
#include <QAudioOutput>
#include <QMediaDevices>
#include <QMediaPlayer>

// デスクトップ実装: QtMultimedia へ委譲する。

struct MediaOutput::Impl {
    QAudioOutput output;
};

MediaOutput::MediaOutput(QObject *parent) : QObject(parent), m_impl(std::make_unique<Impl>()) {
    connect(&m_impl->output, &QAudioOutput::volumeChanged, this, &MediaOutput::volumeChanged);
    connect(&m_impl->output, &QAudioOutput::mutedChanged, this, &MediaOutput::mutedChanged);
    connect(&m_impl->output, &QAudioOutput::deviceChanged, this, &MediaOutput::deviceChanged);
}
MediaOutput::~MediaOutput() = default;
qreal MediaOutput::volume() const { return m_impl->output.volume(); }
void MediaOutput::setVolume(qreal value) { m_impl->output.setVolume(value); }
bool MediaOutput::muted() const { return m_impl->output.isMuted(); }
void MediaOutput::setMuted(bool value) { m_impl->output.setMuted(value); }
QVariant MediaOutput::device() const { return QVariant::fromValue(m_impl->output.device()); }
void MediaOutput::setDevice(const QVariant &value) {
    if (value.canConvert<QAudioDevice>()) {
        m_impl->output.setDevice(value.value<QAudioDevice>());
    }
}
void *MediaOutput::nativeHandle() const { return &m_impl->output; }

struct MediaDevices::Impl {
    QList<QAudioDevice> devices;
    QAudioDevice defaultDevice;
    Impl() {
        devices = QMediaDevices::audioOutputs();
        defaultDevice = QMediaDevices::defaultAudioOutput();
    }
};

MediaDevices::MediaDevices(QObject *parent) : QObject(parent), m_impl(std::make_unique<Impl>()) {}
MediaDevices::~MediaDevices() = default;
QVariantList MediaDevices::audioOutputs() const {
    QVariantList result;
    result.reserve(m_impl->devices.size());
    for (const QAudioDevice &device : m_impl->devices) {
        result.append(QVariant::fromValue(device));
    }
    return result;
}
QVariant MediaDevices::defaultAudioOutput() const {
    return QVariant::fromValue(m_impl->defaultDevice);
}

struct MediaPlayer::Impl {
    QMediaPlayer player;
    QObject *audioOutputObject = nullptr;
};

MediaPlayer::MediaPlayer(QObject *parent) : QObject(parent), m_impl(std::make_unique<Impl>()) {
    connect(&m_impl->player, &QMediaPlayer::playbackStateChanged, this,
            [this](QMediaPlayer::PlaybackState) { emit playbackStateChanged(); });
    connect(&m_impl->player, &QMediaPlayer::mediaStatusChanged, this,
            [this](QMediaPlayer::MediaStatus) { emit mediaStatusChanged(); });
    connect(&m_impl->player, &QMediaPlayer::positionChanged, this,
            [this](qint64) { emit positionChanged(); });
    connect(&m_impl->player, &QMediaPlayer::durationChanged, this,
            [this](qint64) { emit durationChanged(); });
    connect(&m_impl->player, &QMediaPlayer::errorOccurred, this,
            [this](QMediaPlayer::Error error, const QString &message) {
                emit errorOccurred(static_cast<int>(error), message);
            });
}
MediaPlayer::~MediaPlayer() = default;
QUrl MediaPlayer::source() const { return m_impl->player.source(); }
void MediaPlayer::setSource(const QUrl &value) {
    if (m_impl->player.source() == value) {
        return;
    }
    m_impl->player.setSource(value);
    emit sourceChanged();
}
QObject *MediaPlayer::audioOutput() const { return m_impl->audioOutputObject; }
void MediaPlayer::setAudioOutput(QObject *value) {
    m_impl->audioOutputObject = value;
    if (auto *output = qobject_cast<MediaOutput *>(value)) {
        m_impl->player.setAudioOutput(static_cast<QAudioOutput *>(output->nativeHandle()));
    }
    emit audioOutputChanged();
}
int MediaPlayer::playbackState() const { return static_cast<int>(m_impl->player.playbackState()); }
int MediaPlayer::mediaStatus() const { return static_cast<int>(m_impl->player.mediaStatus()); }
QString MediaPlayer::errorString() const { return m_impl->player.errorString(); }
qreal MediaPlayer::position() const { return static_cast<qreal>(m_impl->player.position()); }
void MediaPlayer::setPosition(qreal value) {
    m_impl->player.setPosition(static_cast<qint64>(value));
}
qreal MediaPlayer::duration() const { return static_cast<qreal>(m_impl->player.duration()); }
void MediaPlayer::play() { m_impl->player.play(); }
void MediaPlayer::pause() { m_impl->player.pause(); }
void MediaPlayer::stop() { m_impl->player.stop(); }
