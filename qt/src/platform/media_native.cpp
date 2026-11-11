#include "platform/media.h"

#include <QAudioDevice>
#include <QAudioOutput>
#include <QMediaDevices>
#include <QMediaPlayer>


// マルチメディアの初期化は起動を遅らせるので、QAudioOutput は再生に使うときに作る。
struct MediaOutput::Impl {
    std::unique_ptr<QAudioOutput> output;
    qreal volume = 1.0;
    bool muted = false;
    QAudioDevice device;

    QAudioOutput *get(MediaOutput *owner) {
        if (output)
            return output.get();
        output = std::make_unique<QAudioOutput>();
        output->setVolume(volume);
        output->setMuted(muted);
        if (!device.isNull())
            output->setDevice(device);
        QObject::connect(output.get(), &QAudioOutput::volumeChanged, owner, &MediaOutput::volumeChanged);
        QObject::connect(output.get(), &QAudioOutput::mutedChanged, owner, &MediaOutput::mutedChanged);
        QObject::connect(output.get(), &QAudioOutput::deviceChanged, owner, &MediaOutput::deviceChanged);
        return output.get();
    }
};

MediaOutput::MediaOutput(QObject *parent) : QObject(parent), m_impl(std::make_unique<Impl>()) {}
MediaOutput::~MediaOutput() = default;
qreal MediaOutput::volume() const { return m_impl->output ? m_impl->output->volume() : m_impl->volume; }
void MediaOutput::setVolume(qreal value) {
    if (m_impl->output) {
        m_impl->output->setVolume(value);
    } else if (m_impl->volume != value) {
        m_impl->volume = value;
        emit volumeChanged();
    }
}
bool MediaOutput::muted() const { return m_impl->output ? m_impl->output->isMuted() : m_impl->muted; }
void MediaOutput::setMuted(bool value) {
    if (m_impl->output) {
        m_impl->output->setMuted(value);
    } else if (m_impl->muted != value) {
        m_impl->muted = value;
        emit mutedChanged();
    }
}
QVariant MediaOutput::device() const {
    return QVariant::fromValue(m_impl->output ? m_impl->output->device() : m_impl->device);
}
void MediaOutput::setDevice(const QVariant &value) {
    if (!value.canConvert<QAudioDevice>())
        return;
    const QAudioDevice device = value.value<QAudioDevice>();
    if (m_impl->output) {
        m_impl->output->setDevice(device);
    } else if (m_impl->device != device) {
        m_impl->device = device;
        emit deviceChanged();
    }
}
void *MediaOutput::nativeHandle() const { return m_impl->get(const_cast<MediaOutput *>(this)); }

// 出力先の列挙は起動を遅らせるので、初めて読まれたときに行う。
struct MediaDevices::Impl {
    QList<QAudioDevice> devices;
    QAudioDevice defaultDevice;
    bool loaded = false;
    void load() {
        if (loaded)
            return;
        loaded = true;
        devices = QMediaDevices::audioOutputs();
        defaultDevice = QMediaDevices::defaultAudioOutput();
    }
};

MediaDevices::MediaDevices(QObject *parent) : QObject(parent), m_impl(std::make_unique<Impl>()) {}
MediaDevices::~MediaDevices() = default;
QVariantList MediaDevices::audioOutputs() const {
    m_impl->load();
    QVariantList result;
    result.reserve(m_impl->devices.size());
    for (const QAudioDevice &device : m_impl->devices) {
        result.append(QVariant::fromValue(device));
    }
    return result;
}
QVariant MediaDevices::defaultAudioOutput() const {
    m_impl->load();
    return QVariant::fromValue(m_impl->defaultDevice);
}

// QMediaPlayer も、初めて音を扱うときに作る。それまでは止まっている状態を返す。
struct MediaPlayer::Impl {
    std::unique_ptr<QMediaPlayer> player;
    QObject *audioOutputObject = nullptr;

    QMediaPlayer *get(MediaPlayer *owner) {
        if (player)
            return player.get();
        player = std::make_unique<QMediaPlayer>();
        QObject::connect(player.get(), &QMediaPlayer::playbackStateChanged, owner,
                         [owner](QMediaPlayer::PlaybackState) { emit owner->playbackStateChanged(); });
        QObject::connect(player.get(), &QMediaPlayer::mediaStatusChanged, owner,
                         [owner](QMediaPlayer::MediaStatus) { emit owner->mediaStatusChanged(); });
        QObject::connect(player.get(), &QMediaPlayer::positionChanged, owner,
                         [owner](qint64) { emit owner->positionChanged(); });
        QObject::connect(player.get(), &QMediaPlayer::durationChanged, owner,
                         [owner](qint64) { emit owner->durationChanged(); });
        QObject::connect(player.get(), &QMediaPlayer::errorOccurred, owner,
                         [owner](QMediaPlayer::Error error, const QString &message) {
                             emit owner->errorOccurred(static_cast<int>(error), message);
                         });
        if (auto *output = qobject_cast<MediaOutput *>(audioOutputObject))
            player->setAudioOutput(static_cast<QAudioOutput *>(output->nativeHandle()));
        return player.get();
    }
};

MediaPlayer::MediaPlayer(QObject *parent) : QObject(parent), m_impl(std::make_unique<Impl>()) {}
MediaPlayer::~MediaPlayer() = default;
QUrl MediaPlayer::source() const { return m_impl->player ? m_impl->player->source() : QUrl(); }
void MediaPlayer::setSource(const QUrl &value) {
    if (source() == value)
        return;
    m_impl->get(this)->setSource(value);
    emit sourceChanged();
}
QObject *MediaPlayer::audioOutput() const { return m_impl->audioOutputObject; }
void MediaPlayer::setAudioOutput(QObject *value) {
    m_impl->audioOutputObject = value;
    if (m_impl->player) {
        if (auto *output = qobject_cast<MediaOutput *>(value))
            m_impl->player->setAudioOutput(static_cast<QAudioOutput *>(output->nativeHandle()));
    }
    emit audioOutputChanged();
}
int MediaPlayer::playbackState() const {
    return static_cast<int>(m_impl->player ? m_impl->player->playbackState() : QMediaPlayer::StoppedState);
}
int MediaPlayer::mediaStatus() const {
    return static_cast<int>(m_impl->player ? m_impl->player->mediaStatus() : QMediaPlayer::NoMedia);
}
QString MediaPlayer::errorString() const { return m_impl->player ? m_impl->player->errorString() : QString(); }
qreal MediaPlayer::position() const { return m_impl->player ? static_cast<qreal>(m_impl->player->position()) : 0; }
void MediaPlayer::setPosition(qreal value) {
    if (m_impl->player)
        m_impl->player->setPosition(static_cast<qint64>(value));
}
qreal MediaPlayer::duration() const { return m_impl->player ? static_cast<qreal>(m_impl->player->duration()) : 0; }
void MediaPlayer::play() { m_impl->get(this)->play(); }
void MediaPlayer::pause() {
    if (m_impl->player)
        m_impl->player->pause();
}
void MediaPlayer::stop() {
    if (m_impl->player)
        m_impl->player->stop();
}
