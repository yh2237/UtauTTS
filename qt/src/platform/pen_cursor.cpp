#include "platform/pen_cursor.h"

#include <QCursor>
#include <QGuiApplication>
#include <QPainter>
#include <QPainterPath>
#include <QPixmap>
#include <QtQml>
#include <algorithm>
#include <cmath>

namespace {

#ifndef UTAUTTS_WASM
// 32×32の論理座標で、ペン先（左下）を指す位置にする。
QCursor makePenCursor() {
    const qreal dpr = std::max<qreal>(1.0, std::ceil(qGuiApp->devicePixelRatio()));
    QPixmap pixmap(QSize(32, 32) * dpr);
    pixmap.setDevicePixelRatio(dpr);
    pixmap.fill(Qt::transparent);

    QPainter painter(&pixmap);
    painter.setRenderHint(QPainter::Antialiasing);
    painter.translate(3, 29);
    painter.rotate(-45);
    // ペン先からの長さ方向をxとして描く。
    QPainterPath body;
    body.addRoundedRect(QRectF(9, -4, 25, 8), 1.5, 1.5);
    QPainterPath nib;
    nib.moveTo(0, 0);
    nib.lineTo(9, -4);
    nib.lineTo(9, 4);
    nib.closeSubpath();
    const QPen outline(QColor(0, 0, 0), 1.4, Qt::SolidLine, Qt::RoundCap, Qt::RoundJoin);
    painter.setPen(outline);
    painter.setBrush(QColor(255, 255, 255));
    painter.drawPath(nib);
    painter.drawPath(body);
    painter.setBrush(QColor(0, 0, 0));
    painter.setPen(Qt::NoPen);
    QPainterPath tip;
    tip.moveTo(0, 0);
    tip.lineTo(3.5, -1.6);
    tip.lineTo(3.5, 1.6);
    tip.closeSubpath();
    painter.drawPath(tip);
    painter.setPen(outline);
    painter.drawLine(QPointF(29, -4), QPointF(29, 4));
    painter.end();
    return QCursor(pixmap, 3, 29);
}
#endif

} // namespace

PenCursorArea::PenCursorArea(QQuickItem *parent)
    : QQuickItem(parent) {
}

void PenCursorArea::setActive(bool active) {
    if (m_active == active)
        return;
    m_active = active;
    if (active) {
#ifdef UTAUTTS_WASM
        // ブラウザでは画像のカーソルが使えないため十字にする。
        setCursor(Qt::CrossCursor);
#else
        static const QCursor pen = makePenCursor();
        setCursor(pen);
#endif
    } else {
        unsetCursor();
    }
    emit activeChanged();
}

void registerPenCursorArea() {
    qmlRegisterType<PenCursorArea>("UtauTTS.Platform", 1, 0, "PenCursorArea");
}
