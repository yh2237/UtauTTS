#pragma once

#include <QObject>
#include <QString>

// プラットフォーム能力を QML へ公開するシングルトン。
// QML 側に wasm/desktop 判定を散在させず、ここへ一本化する。
// QML: import UtauTTS.Platform 1.0 -> Platform.isWeb / isDesktop / isMobile
class Platform : public QObject {
    Q_OBJECT
    Q_PROPERTY(bool isDesktop READ isDesktop CONSTANT)
    Q_PROPERTY(bool isWeb READ isWeb CONSTANT)
    Q_PROPERTY(bool isMobile READ isMobile CONSTANT)
    Q_PROPERTY(bool hasNativeFileDialog READ hasNativeFileDialog CONSTANT)
    Q_PROPERTY(QString name READ name CONSTANT)
public:
    explicit Platform(QObject *parent = nullptr);

    bool isDesktop() const;
    bool isWeb() const;
    bool isMobile() const;
    bool hasNativeFileDialog() const;
    QString name() const;
};

// QML の "UtauTTS.Platform" モジュールへ Platform シングルトンを登録する。
void registerPlatformSingleton();
