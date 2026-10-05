#pragma once

#include <QObject>
#include <QUrl>
#include <QVariant>
#include <QVariantList>
#include <memory>

// デスクトップはQtMultimedia、wasmはHTML Audioを使う。

class MediaOutput : public QObject {
    Q_OBJECT
    Q_PROPERTY(qreal volume READ volume WRITE setVolume NOTIFY volumeChanged)
    Q_PROPERTY(bool muted READ muted WRITE setMuted NOTIFY mutedChanged)
    Q_PROPERTY(QVariant device READ device WRITE setDevice NOTIFY deviceChanged)
public:
    explicit MediaOutput(QObject *parent = nullptr);
    ~MediaOutput() override;

    qreal volume() const;
    void setVolume(qreal value);
    bool muted() const;
    void setMuted(bool value);
    QVariant device() const;
    void setDevice(const QVariant &value);

    // デスクトップでは QAudioOutput*、wasm では nullptr を返す。
    void *nativeHandle() const;

signals:
    void volumeChanged();
    void mutedChanged();
    void deviceChanged();

private:
    struct Impl;
    std::unique_ptr<Impl> m_impl;
};

class MediaDevices : public QObject {
    Q_OBJECT
    Q_PROPERTY(QVariantList audioOutputs READ audioOutputs NOTIFY audioOutputsChanged)
    Q_PROPERTY(QVariant defaultAudioOutput READ defaultAudioOutput NOTIFY audioOutputsChanged)
public:
    explicit MediaDevices(QObject *parent = nullptr);
    ~MediaDevices() override;

    QVariantList audioOutputs() const;
    QVariant defaultAudioOutput() const;

signals:
    void audioOutputsChanged();

private:
    struct Impl;
    std::unique_ptr<Impl> m_impl;
};

class MediaPlayer : public QObject {
    Q_OBJECT
    Q_PROPERTY(QUrl source READ source WRITE setSource NOTIFY sourceChanged)
    Q_PROPERTY(QObject *audioOutput READ audioOutput WRITE setAudioOutput NOTIFY audioOutputChanged)
    Q_PROPERTY(int playbackState READ playbackState NOTIFY playbackStateChanged)
    Q_PROPERTY(int mediaStatus READ mediaStatus NOTIFY mediaStatusChanged)
    Q_PROPERTY(qreal position READ position WRITE setPosition NOTIFY positionChanged)
    Q_PROPERTY(qreal duration READ duration NOTIFY durationChanged)
    Q_PROPERTY(QString errorString READ errorString NOTIFY errorOccurred)
public:
    enum PlaybackState { StoppedState = 0, PlayingState = 1, PausedState = 2 };
    Q_ENUM(PlaybackState)
    enum MediaStatus {
        NoMedia = 0,
        LoadingMedia = 1,
        LoadedMedia = 2,
        BufferedMedia = 3,
        StalledMedia = 4,
        EndOfMedia = 5,
        InvalidMedia = 6
    };
    Q_ENUM(MediaStatus)

    explicit MediaPlayer(QObject *parent = nullptr);
    ~MediaPlayer() override;

    QUrl source() const;
    void setSource(const QUrl &value);
    QObject *audioOutput() const;
    void setAudioOutput(QObject *value);
    int playbackState() const;
    int mediaStatus() const;
    qreal position() const;
    void setPosition(qreal value);
    qreal duration() const;
    QString errorString() const;

    Q_INVOKABLE void play();
    Q_INVOKABLE void pause();
    Q_INVOKABLE void stop();

signals:
    void sourceChanged();
    void audioOutputChanged();
    void playbackStateChanged();
    void mediaStatusChanged();
    void positionChanged();
    void durationChanged();
    void errorOccurred(int error, const QString &errorString);

private:
    struct Impl;
    std::unique_ptr<Impl> m_impl;
};

void registerMediaTypes();
