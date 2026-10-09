#pragma once

#include <QQuickItem>

// 有効な間だけペンのカーソルを出す透明なItem。マウスの操作は受けず、下のMouseAreaへそのまま届く。
class PenCursorArea : public QQuickItem {
    Q_OBJECT
    Q_PROPERTY(bool active READ active WRITE setActive NOTIFY activeChanged)
public:
    explicit PenCursorArea(QQuickItem *parent = nullptr);

    bool active() const { return m_active; }
    void setActive(bool active);

signals:
    void activeChanged();

private:
    bool m_active = false;
};

void registerPenCursorArea();
