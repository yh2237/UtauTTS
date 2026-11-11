#include "fusionstyle.h"

#include <QCoreApplication>
#include <QQuickItem>
#include <QStyle>
#include <QStyleFactory>

namespace {
struct Role {
    const char *name;
    QPalette::ColorRole role;
};

constexpr Role roles[] = {
    {"window", QPalette::Window},
    {"windowText", QPalette::WindowText},
    {"base", QPalette::Base},
    {"alternateBase", QPalette::AlternateBase},
    {"text", QPalette::Text},
    {"button", QPalette::Button},
    {"buttonText", QPalette::ButtonText},
    {"brightText", QPalette::BrightText},
    {"light", QPalette::Light},
    {"midlight", QPalette::Midlight},
    {"mid", QPalette::Mid},
    {"dark", QPalette::Dark},
    {"shadow", QPalette::Shadow},
    {"highlight", QPalette::Highlight},
    {"highlightedText", QPalette::HighlightedText},
    {"placeholderText", QPalette::PlaceholderText},
    {"toolTipBase", QPalette::ToolTipBase},
    {"toolTipText", QPalette::ToolTipText},
    {"link", QPalette::Link},
    {"linkVisited", QPalette::LinkVisited},
};

void copyGroup(const QObject *group, QPalette::ColorGroup target, QPalette &palette) {
    if (!group)
        return;
    for (const Role &role : roles) {
        const QVariant value = group->property(role.name);
        if (value.canConvert<QColor>())
            palette.setColor(target, role.role, value.value<QColor>());
    }
}
}

namespace FusionStyle {
QStyle *style() {
    static QStyle *fusion = [] {
        QStyle *created = QStyleFactory::create(QStringLiteral("Fusion"));
        created->setParent(QCoreApplication::instance());
        return created;
    }();
    return fusion;
}

QPalette palette(const QQuickItem *item) {
    QPalette result = style()->standardPalette();
    const auto *source = item ? item->property("palette").value<QObject *>() : nullptr;
    if (!source)
        return result;
    copyGroup(source->property("active").value<QObject *>(), QPalette::Active, result);
    copyGroup(source->property("inactive").value<QObject *>(), QPalette::Inactive, result);
    copyGroup(source->property("disabled").value<QObject *>(), QPalette::Disabled, result);
    return result;
}
}
