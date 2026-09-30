#pragma once

#include <QObject>
#include <QString>

// 環境とレイアウトの判定を共通QMLへ公開する。
class Platform : public QObject {
    Q_OBJECT
    Q_PROPERTY(bool isDesktop READ isDesktop CONSTANT)
    Q_PROPERTY(bool isWeb READ isWeb CONSTANT)
    Q_PROPERTY(bool isMobile READ isMobile NOTIFY layoutChanged)
    Q_PROPERTY(bool hasNativeFileDialog READ hasNativeFileDialog CONSTANT)
    Q_PROPERTY(bool hasExternalTools READ hasExternalTools CONSTANT)
    Q_PROPERTY(bool hasDiffsinger READ hasDiffsinger CONSTANT)
    Q_PROPERTY(QString name READ name NOTIFY layoutChanged)
public:
    explicit Platform(QObject *parent = nullptr);

    bool isDesktop() const;
    bool isWeb() const;
    bool isMobile() const;
    bool hasNativeFileDialog() const;
    bool hasExternalTools() const;
    bool hasDiffsinger() const;
    QString name() const;
    Q_INVOKABLE void updateViewportWidth(qreal width);

signals:
    void layoutChanged();

private:
    int m_mobileOverride = -1;
    bool m_mobile = false;
};

void registerPlatformSingleton();
