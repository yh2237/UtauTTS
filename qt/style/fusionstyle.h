#pragma once

#include <QPalette>

class QQuickItem;
class QStyle;

namespace FusionStyle {
QStyle *style();
QPalette palette(const QQuickItem *item);
}
