#include "styleitem.h"

#include "fusionstyle.h"

#include <QAbstractSpinBox>
#include <cmath>
#include <QFrame>
#include <QPainter>
#include <QSlider>
#include <QStyle>
#include <QStyleOption>
#include <QTabBar>

namespace {
constexpr int resolution = 1000000;

QStyle *fusion() {
    return FusionStyle::style();
}

QStyle::SubControl toStyleSubControl(StyleItem::Element element, StyleItem::SubControl control) {
    switch (element) {
    case StyleItem::ScrollBar:
        switch (control) {
        case StyleItem::SubLine: return QStyle::SC_ScrollBarSubLine;
        case StyleItem::AddLine: return QStyle::SC_ScrollBarAddLine;
        case StyleItem::SubPage: return QStyle::SC_ScrollBarSubPage;
        case StyleItem::AddPage: return QStyle::SC_ScrollBarAddPage;
        case StyleItem::Handle: return QStyle::SC_ScrollBarSlider;
        case StyleItem::Groove: return QStyle::SC_ScrollBarGroove;
        default: return QStyle::SC_None;
        }
    case StyleItem::SpinBox:
        switch (control) {
        case StyleItem::Up: return QStyle::SC_SpinBoxUp;
        case StyleItem::Down: return QStyle::SC_SpinBoxDown;
        case StyleItem::EditField: return QStyle::SC_SpinBoxEditField;
        default: return QStyle::SC_None;
        }
    case StyleItem::ComboBox:
        switch (control) {
        case StyleItem::Arrow: return QStyle::SC_ComboBoxArrow;
        case StyleItem::EditField: return QStyle::SC_ComboBoxEditField;
        default: return QStyle::SC_None;
        }
    case StyleItem::Slider:
        switch (control) {
        case StyleItem::Handle: return QStyle::SC_SliderHandle;
        case StyleItem::Groove: return QStyle::SC_SliderGroove;
        default: return QStyle::SC_None;
        }
    default:
        return QStyle::SC_None;
    }
}

StyleItem::SubControl fromStyleSubControl(StyleItem::Element element, QStyle::SubControl control) {
    for (const StyleItem::SubControl candidate : {StyleItem::SubLine, StyleItem::AddLine, StyleItem::SubPage,
                                                  StyleItem::AddPage, StyleItem::Handle, StyleItem::Groove,
                                                  StyleItem::Up, StyleItem::Down, StyleItem::Arrow,
                                                  StyleItem::EditField}) {
        if (toStyleSubControl(element, candidate) == control)
            return candidate;
    }
    return StyleItem::None;
}

QStyle::ComplexControl complexControl(StyleItem::Element element) {
    switch (element) {
    case StyleItem::ScrollBar: return QStyle::CC_ScrollBar;
    case StyleItem::SpinBox: return QStyle::CC_SpinBox;
    case StyleItem::ComboBox: return QStyle::CC_ComboBox;
    case StyleItem::Slider: return QStyle::CC_Slider;
    default: return QStyle::CC_CustomBase;
    }
}
}

StyleItem::StyleItem(QQuickItem *parent)
    : QQuickPaintedItem(parent) {
    connect(this, &StyleItem::stateChanged, this, [this] { update(); });
    connect(this, &StyleItem::metricsChanged, this, [this] { update(); });
    connect(this, &QQuickItem::enabledChanged, this, [this] { update(); });
    connect(this, &QQuickItem::paletteChanged, this, [this] { update(); });
    updateMetrics();
}

void StyleItem::geometryChange(const QRectF &newGeometry, const QRectF &oldGeometry) {
    QQuickPaintedItem::geometryChange(newGeometry, oldGeometry);
    if (newGeometry.size() != oldGeometry.size()) {
        update();
        emit geometryChanged();
    }
}

void StyleItem::initOption(QStyleOption &option) const {
    option.rect = QRect(0, 0, qRound(width()), qRound(height()));
    option.palette = FusionStyle::palette(this);
    option.palette.setCurrentColorGroup(isEnabled() ? QPalette::Active : QPalette::Disabled);
    option.fontMetrics = QFontMetrics(m_font);
    option.direction = Qt::LeftToRight;
    option.state = QStyle::State_Active;
    if (isEnabled())
        option.state |= QStyle::State_Enabled;
    if (m_hovered && isEnabled())
        option.state |= QStyle::State_MouseOver;
    if (m_focused)
        option.state |= QStyle::State_HasFocus;
    if (m_orientation == Qt::Horizontal)
        option.state |= QStyle::State_Horizontal;
}

QStyleOptionSlider StyleItem::sliderOption() const {
    QStyleOptionSlider option;
    initOption(option);
    option.orientation = m_orientation;
    option.minimum = 0;
    if (m_element == ScrollBar) {
        option.pageStep = qMax(1, qRound(qBound(0.0, m_size, 1.0) * resolution));
        option.maximum = qMax(0, resolution - option.pageStep);
    } else {
        option.pageStep = resolution / 10;
        option.maximum = resolution;
    }
    option.singleStep = qMax(1, option.pageStep / 10);
    option.sliderPosition = qBound(0, qRound(m_value * resolution), option.maximum);
    option.sliderValue = option.sliderPosition;
    option.subControls = m_element == Slider
            ? QStyle::SubControls(QStyle::SC_SliderGroove | QStyle::SC_SliderHandle)
            : QStyle::SubControls(QStyle::SC_All);
    option.tickPosition = QSlider::NoTicks;
    option.upsideDown = m_element == Slider && m_orientation == Qt::Vertical;
    return option;
}

namespace {
QMarginsF splitMargins(QSize total, QSize content) {
    const qreal horizontal = qMax(0, total.width() - content.width());
    const qreal vertical = qMax(0, total.height() - content.height());
    const qreal left = std::floor(horizontal / 2);
    const qreal top = std::floor(vertical / 2);
    return QMarginsF(left, top, horizontal - left, vertical - top);
}

QMarginsF fieldMargins(QSize total, QRect field, int textHeight, int textMargin) {
    const qreal left = field.x() + textMargin;
    const qreal right = total.width() - (field.x() + field.width()) + textMargin;
    const qreal free = qMax(0, total.height() - textHeight);
    const qreal top = std::floor(free / 2);
    return QMarginsF(left, top, right, free - top);
}
}

void StyleItem::updateMetrics() {
    QStyle *style = fusion();
    const int textHeight = QFontMetrics(m_font).height();
    constexpr int probeWidth = 100;

    QMarginsF padding;
    qreal minimumWidth = 0;
    qreal spacing = 0;
    qreal extent = 0;
    qreal handleLength = 0;

    switch (m_element) {
    case Button: {
        QStyleOptionButton option;
        initOption(option);
        const QSize content(probeWidth, textHeight);
        padding = splitMargins(style->sizeFromContents(QStyle::CT_PushButton, &option, content), content);
        minimumWidth = 80;
        spacing = 4;
        extent = style->pixelMetric(QStyle::PM_ButtonIconSize);
        break;
    }
    case ToolButton: {
        QStyleOptionToolButton option;
        initOption(option);
        const QSize content(probeWidth, textHeight);
        padding = splitMargins(style->sizeFromContents(QStyle::CT_ToolButton, &option, content), content);
        spacing = 4;
        extent = style->pixelMetric(QStyle::PM_ButtonIconSize);
        break;
    }
    case CheckBox:
    case RadioButton: {
        QStyleOptionButton option;
        initOption(option);
        const bool radio = m_element == RadioButton;
        extent = style->pixelMetric(radio ? QStyle::PM_ExclusiveIndicatorWidth : QStyle::PM_IndicatorWidth);
        spacing = style->pixelMetric(radio ? QStyle::PM_RadioButtonLabelSpacing : QStyle::PM_CheckBoxLabelSpacing);
        option.text = QStringLiteral(" ");
        const QSize total = style->sizeFromContents(radio ? QStyle::CT_RadioButton : QStyle::CT_CheckBox,
                                                    &option, QSize(0, textHeight));
        padding = splitMargins(QSize(0, total.height()), QSize(0, textHeight));
        break;
    }
    case ComboBox: {
        QStyleOptionComboBox option;
        initOption(option);
        option.editable = m_editable;
        option.frame = !m_flat;
        option.subControls = QStyle::SC_All;
        const QSize total = style->sizeFromContents(QStyle::CT_ComboBox, &option,
                                                    QSize(probeWidth, qMax(textHeight, 14) + 2));
        option.rect = QRect(QPoint(), total);
        const QRect field = style->subControlRect(QStyle::CC_ComboBox, &option, QStyle::SC_ComboBoxEditField);
        padding = fieldMargins(total, field, textHeight, 1);
        break;
    }
    case SpinBox: {
        QStyleOptionFrame lineOption;
        initOption(lineOption);
        lineOption.lineWidth = 0;
        const int lineHeight = style->sizeFromContents(
                QStyle::CT_LineEdit, &lineOption, QSize(probeWidth, qMax(textHeight, 14) + 2)).height();
        QStyleOptionSpinBox option;
        initOption(option);
        option.frame = true;
        option.buttonSymbols = QAbstractSpinBox::UpDownArrows;
        option.subControls = QStyle::SC_SpinBoxFrame | QStyle::SC_SpinBoxEditField
                | QStyle::SC_SpinBoxUp | QStyle::SC_SpinBoxDown;
        const QSize total = style->sizeFromContents(QStyle::CT_SpinBox, &option, QSize(probeWidth, lineHeight));
        option.rect = QRect(QPoint(), total);
        const QRect field = style->subControlRect(QStyle::CC_SpinBox, &option, QStyle::SC_SpinBoxEditField);
        padding = fieldMargins(total, field, textHeight, 2);
        break;
    }
    case TextField: {
        QStyleOptionFrame option;
        initOption(option);
        option.lineWidth = style->pixelMetric(QStyle::PM_DefaultFrameWidth);
        const QSize content(probeWidth, textHeight);
        const QSize total = style->sizeFromContents(QStyle::CT_LineEdit, &option,
                                                    QSize(probeWidth + 4, qMax(textHeight, 14) + 2));
        padding = splitMargins(total, content);
        break;
    }
    case Frame:
    case TabFrame: {
        const qreal frame = style->pixelMetric(QStyle::PM_DefaultFrameWidth);
        padding = QMarginsF(frame, frame, frame, frame);
        break;
    }
    case Slider:
        extent = style->pixelMetric(QStyle::PM_SliderThickness);
        handleLength = style->pixelMetric(QStyle::PM_SliderLength);
        break;
    case ProgressBar:
        padding = QMarginsF(0, 4, 0, 4);
        break;
    case ScrollBar:
        extent = style->pixelMetric(QStyle::PM_ScrollBarExtent);
        handleLength = style->pixelMetric(QStyle::PM_ScrollBarSliderMin);
        break;
    case Tab: {
        QStyleOptionTab option;
        initOption(option);
        option.shape = QTabBar::RoundedNorth;
        const QSize content(probeWidth, textHeight);
        const QSize spaced = content + QSize(style->pixelMetric(QStyle::PM_TabBarTabHSpace),
                                             style->pixelMetric(QStyle::PM_TabBarTabVSpace));
        padding = splitMargins(style->sizeFromContents(QStyle::CT_TabBarTab, &option, spaced), content);
        break;
    }
    case MenuBar:
        spacing = style->pixelMetric(QStyle::PM_MenuBarItemSpacing);
        break;
    case MenuBarItem: {
        QStyleOptionMenuItem option;
        initOption(option);
        option.menuItemType = QStyleOptionMenuItem::Normal;
        const QSize content(probeWidth, textHeight);
        padding = splitMargins(style->sizeFromContents(QStyle::CT_MenuBarItem, &option, content), content);
        extent = style->pixelMetric(QStyle::PM_SmallIconSize);
        break;
    }
    case MenuPanel:
        padding = QMarginsF(1, 1, 1, 1);
        break;
    case MenuItem: {
        QStyleOptionMenuItem option;
        initOption(option);
        option.menuItemType = QStyleOptionMenuItem::Normal;
        option.font = m_font;
        const QSize content(probeWidth, textHeight);
        padding = splitMargins(style->sizeFromContents(QStyle::CT_MenuItem, &option, content), content);
        const qreal itemHeight = textHeight + padding.top() + padding.bottom();
        const qreal checkColumn = qMax<qreal>(int(itemHeight * 0.79), 21);
        const qreal textOffset = 4 + checkColumn + 3;
        const qreal horizontal = padding.left() + padding.right();
        padding.setLeft(textOffset);
        padding.setRight(qMax<qreal>(0, horizontal - textOffset));
        minimumWidth = 120;
        extent = style->pixelMetric(QStyle::PM_SmallIconSize);
        break;
    }
    case MenuSeparator: {
        QStyleOptionMenuItem option;
        initOption(option);
        option.menuItemType = QStyleOptionMenuItem::Separator;
        padding = splitMargins(style->sizeFromContents(QStyle::CT_MenuItem, &option, QSize()), QSize());
        break;
    }
    case ToolTip: {
        const qreal margin = 1 + style->pixelMetric(QStyle::PM_ToolTipLabelFrameWidth);
        padding = QMarginsF(margin, margin, margin, margin);
        break;
    }
    case Splitter:
        extent = style->pixelMetric(QStyle::PM_SplitterWidth);
        break;
    case ItemView: {
        const qreal margin = style->pixelMetric(QStyle::PM_FocusFrameHMargin) + 1;
        padding = QMarginsF(margin, 2, margin, 2);
        extent = style->pixelMetric(QStyle::PM_SmallIconSize);
        spacing = margin;
        break;
    }
    case HeaderSection: {
        QStyleOptionHeader option;
        initOption(option);
        option.orientation = Qt::Horizontal;
        option.text = QStringLiteral(" ");
        const QSize content(0, textHeight);
        const QSize total = style->sizeFromContents(QStyle::CT_HeaderSection, &option, QSize());
        const qreal margin = style->pixelMetric(QStyle::PM_HeaderMargin);
        padding = QMarginsF(margin, 0, margin, 0);
        padding.setTop(std::floor(qMax(0, total.height() - content.height()) / 2.0));
        padding.setBottom(qMax(0, total.height() - content.height()) - padding.top());
        break;
    }
    case ToolBar: {
        const qreal margin = style->pixelMetric(QStyle::PM_ToolBarItemMargin)
                + style->pixelMetric(QStyle::PM_ToolBarFrameWidth);
        padding = QMarginsF(margin, margin, margin, margin);
        spacing = style->pixelMetric(QStyle::PM_ToolBarItemSpacing);
        break;
    }
    }

    m_padding = padding;
    m_minimumWidth = minimumWidth;
    m_spacing = spacing;
    m_extent = extent;
    m_handleLength = handleLength;
    emit metricsChanged();
    emit geometryChanged();
}

qreal StyleItem::lineButtonLength() const {
    const QRectF rect = subControlRect(SubLine);
    return m_orientation == Qt::Vertical ? rect.height() : rect.width();
}

int StyleItem::pixelMetric(Metric metric) const {
    switch (metric) {
    case TabBarBaseOverlap: return fusion()->pixelMetric(QStyle::PM_TabBarBaseOverlap);
    case DefaultFrameWidth: return fusion()->pixelMetric(QStyle::PM_DefaultFrameWidth);
    case WindowMargin: {
        QStyleOption option;
        option.state = QStyle::State_Window;
        return fusion()->pixelMetric(QStyle::PM_LayoutLeftMargin, &option);
    }
    case LayoutSpacing: return fusion()->pixelMetric(QStyle::PM_LayoutHorizontalSpacing);
    }
    return 0;
}

QRectF StyleItem::subControlRect(SubControl control) const {
    const QStyle::SubControl styleControl = toStyleSubControl(m_element, control);
    if (styleControl == QStyle::SC_None || width() <= 0 || height() <= 0)
        return {};
    switch (m_element) {
    case ScrollBar:
    case Slider: {
        const QStyleOptionSlider option = sliderOption();
        return fusion()->subControlRect(complexControl(m_element), &option, styleControl);
    }
    case SpinBox: {
        QStyleOptionSpinBox option;
        initOption(option);
        option.frame = true;
        option.buttonSymbols = QAbstractSpinBox::UpDownArrows;
        option.subControls = QStyle::SC_SpinBoxFrame | QStyle::SC_SpinBoxEditField
                | QStyle::SC_SpinBoxUp | QStyle::SC_SpinBoxDown;
        return fusion()->subControlRect(QStyle::CC_SpinBox, &option, styleControl);
    }
    case ComboBox: {
        QStyleOptionComboBox option;
        initOption(option);
        option.editable = m_editable;
        option.frame = !m_flat;
        option.subControls = QStyle::SC_All;
        return fusion()->subControlRect(QStyle::CC_ComboBox, &option, styleControl);
    }
    default:
        return {};
    }
}

StyleItem::SubControl StyleItem::hitTest(qreal x, qreal y) const {
    const QPoint point = QPointF(x, y).toPoint();
    switch (m_element) {
    case ScrollBar:
    case Slider: {
        const QStyleOptionSlider option = sliderOption();
        return fromStyleSubControl(m_element,
                                   fusion()->hitTestComplexControl(complexControl(m_element), &option, point));
    }
    case SpinBox: {
        QStyleOptionSpinBox option;
        initOption(option);
        option.frame = true;
        option.buttonSymbols = QAbstractSpinBox::UpDownArrows;
        option.subControls = QStyle::SC_SpinBoxFrame | QStyle::SC_SpinBoxEditField
                | QStyle::SC_SpinBoxUp | QStyle::SC_SpinBoxDown;
        return fromStyleSubControl(SpinBox, fusion()->hitTestComplexControl(QStyle::CC_SpinBox, &option, point));
    }
    default:
        return None;
    }
}

void StyleItem::paint(QPainter *painter) {
    if (width() <= 0 || height() <= 0)
        return;
    QStyle *style = fusion();
    painter->setFont(m_font);

    switch (m_element) {
    case Button: {
        QStyleOptionButton option;
        initOption(option);
        option.state |= m_pressed ? QStyle::State_Sunken : QStyle::State_Raised;
        if (m_checked)
            option.state |= QStyle::State_On;
        if (m_flat)
            option.features |= QStyleOptionButton::Flat;
        if (m_highlighted)
            option.features |= QStyleOptionButton::DefaultButton;
        style->drawControl(QStyle::CE_PushButtonBevel, &option, painter);
        break;
    }
    case ToolButton: {
        QStyleOptionToolButton option;
        initOption(option);
        option.subControls = QStyle::SC_ToolButton;
        option.activeSubControls = m_pressed ? QStyle::SC_ToolButton : QStyle::SC_None;
        option.toolButtonStyle = Qt::ToolButtonIconOnly;
        option.state |= QStyle::State_Raised;
        if (m_flat)
            option.state |= QStyle::State_AutoRaise;
        if (m_pressed)
            option.state |= QStyle::State_Sunken;
        if (m_checked)
            option.state |= QStyle::State_On;
        style->drawComplexControl(QStyle::CC_ToolButton, &option, painter);
        break;
    }
    case CheckBox:
    case RadioButton: {
        QStyleOptionButton option;
        initOption(option);
        if (m_pressed)
            option.state |= QStyle::State_Sunken;
        option.state |= m_partiallyChecked ? QStyle::State_NoChange
                : m_checked ? QStyle::State_On : QStyle::State_Off;
        style->drawPrimitive(m_element == CheckBox ? QStyle::PE_IndicatorCheckBox
                                                   : QStyle::PE_IndicatorRadioButton,
                             &option, painter);
        break;
    }
    case ComboBox: {
        QStyleOptionComboBox option;
        initOption(option);
        option.editable = m_editable;
        option.frame = !m_flat;
        option.subControls = QStyle::SC_All;
        if (m_pressed) {
            option.state |= QStyle::State_Sunken | QStyle::State_On;
            option.activeSubControls = QStyle::SC_ComboBoxArrow;
        }
        style->drawComplexControl(QStyle::CC_ComboBox, &option, painter);
        break;
    }
    case SpinBox: {
        QStyleOptionSpinBox option;
        initOption(option);
        option.frame = true;
        option.buttonSymbols = QAbstractSpinBox::UpDownArrows;
        option.subControls = QStyle::SC_SpinBoxFrame | QStyle::SC_SpinBoxEditField
                | QStyle::SC_SpinBoxUp | QStyle::SC_SpinBoxDown;
        option.stepEnabled = {};
        if (m_upEnabled)
            option.stepEnabled |= QAbstractSpinBox::StepUpEnabled;
        if (m_downEnabled)
            option.stepEnabled |= QAbstractSpinBox::StepDownEnabled;
        if (m_pressedControl != None) {
            option.activeSubControls = toStyleSubControl(SpinBox, m_pressedControl);
            option.state |= QStyle::State_Sunken;
        } else if (m_hoveredControl != None) {
            option.activeSubControls = toStyleSubControl(SpinBox, m_hoveredControl);
        }
        style->drawComplexControl(QStyle::CC_SpinBox, &option, painter);
        break;
    }
    case TextField: {
        QStyleOptionFrame option;
        initOption(option);
        option.lineWidth = style->pixelMetric(QStyle::PM_DefaultFrameWidth);
        option.midLineWidth = 0;
        option.state |= QStyle::State_Sunken;
        if (!m_editable)
            option.state |= QStyle::State_ReadOnly;
        style->drawPrimitive(QStyle::PE_PanelLineEdit, &option, painter);
        break;
    }
    case Frame: {
        QStyleOptionFrame option;
        initOption(option);
        option.frameShape = QFrame::StyledPanel;
        option.lineWidth = 1;
        option.midLineWidth = 0;
        option.state |= QStyle::State_Sunken;
        style->drawControl(QStyle::CE_ShapedFrame, &option, painter);
        break;
    }
    case Slider:
    case ScrollBar: {
        QStyleOptionSlider option = sliderOption();
        if (m_pressedControl != None) {
            option.activeSubControls = toStyleSubControl(m_element, m_pressedControl);
            option.state |= QStyle::State_Sunken;
        } else if (m_hoveredControl != None) {
            option.activeSubControls = toStyleSubControl(m_element, m_hoveredControl);
        }
        style->drawComplexControl(complexControl(m_element), &option, painter);
        break;
    }
    case ProgressBar: {
        QStyleOptionProgressBar option;
        initOption(option);
        option.minimum = 0;
        option.maximum = m_value < 0 ? 0 : resolution;
        option.progress = m_value < 0 ? 0 : qRound(qBound(0.0, m_value, 1.0) * resolution);
        option.textVisible = false;
        style->drawControl(QStyle::CE_ProgressBarGroove, &option, painter);
        style->drawControl(QStyle::CE_ProgressBarContents, &option, painter);
        break;
    }
    case Tab: {
        QStyleOptionTab option;
        initOption(option);
        option.shape = QTabBar::RoundedNorth;
        switch (m_tabPosition) {
        case Beginning: option.position = QStyleOptionTab::Beginning; break;
        case Middle: option.position = QStyleOptionTab::Middle; break;
        case End: option.position = QStyleOptionTab::End; break;
        case OnlyOneTab: option.position = QStyleOptionTab::OnlyOneTab; break;
        }
        option.selectedPosition = QStyleOptionTab::NotAdjacent;
        // QTabWidget の中のタブと同じく、地を枠の色にそろえる。
        option.features |= QStyleOptionTab::HasFrame;
        if (m_checked)
            option.state |= QStyle::State_Selected;
        style->drawControl(QStyle::CE_TabBarTabShape, &option, painter);
        break;
    }
    case TabFrame: {
        QStyleOptionTabWidgetFrame option;
        initOption(option);
        option.shape = QTabBar::RoundedNorth;
        option.lineWidth = style->pixelMetric(QStyle::PM_DefaultFrameWidth);
        style->drawPrimitive(QStyle::PE_FrameTabWidget, &option, painter);
        break;
    }
    case MenuBar: {
        QStyleOptionMenuItem option;
        initOption(option);
        option.menuItemType = QStyleOptionMenuItem::EmptyArea;
        style->drawControl(QStyle::CE_MenuBarEmptyArea, &option, painter);
        break;
    }
    case MenuBarItem: {
        QStyleOptionMenuItem option;
        initOption(option);
        option.menuItemType = QStyleOptionMenuItem::Normal;
        if (m_highlighted || m_pressed)
            option.state |= QStyle::State_Selected;
        if (m_pressed)
            option.state |= QStyle::State_Sunken;
        style->drawControl(QStyle::CE_MenuBarItem, &option, painter);
        break;
    }
    case MenuPanel: {
        QStyleOptionFrame option;
        initOption(option);
        option.lineWidth = style->pixelMetric(QStyle::PM_MenuPanelWidth);
        style->drawPrimitive(QStyle::PE_PanelMenu, &option, painter);
        style->drawPrimitive(QStyle::PE_FrameMenu, &option, painter);
        break;
    }
    case MenuItem:
    case MenuSeparator: {
        QStyleOptionMenuItem option;
        initOption(option);
        option.state &= ~QStyle::State_MouseOver;
        option.menuItemType = m_element == MenuSeparator ? QStyleOptionMenuItem::Separator
                : m_hasSubmenu ? QStyleOptionMenuItem::SubMenu : QStyleOptionMenuItem::Normal;
        option.checkType = m_checkable ? QStyleOptionMenuItem::NonExclusive
                                       : QStyleOptionMenuItem::NotCheckable;
        option.checked = m_checked;
        option.maxIconWidth = 0;
        option.reservedShortcutWidth = 0;
        option.font = m_font;
        if (m_highlighted && isEnabled())
            option.state |= QStyle::State_Selected;
        style->drawControl(QStyle::CE_MenuItem, &option, painter);
        break;
    }
    case ToolTip: {
        QStyleOptionFrame option;
        initOption(option);
        style->drawPrimitive(QStyle::PE_PanelTipLabel, &option, painter);
        break;
    }
    case Splitter: {
        QStyleOption option;
        initOption(option);
        if (m_pressed)
            option.state |= QStyle::State_Sunken;
        style->drawControl(QStyle::CE_Splitter, &option, painter);
        break;
    }
    case ItemView: {
        QStyleOptionViewItem option;
        initOption(option);
        option.showDecorationSelected = true;
        option.viewItemPosition = QStyleOptionViewItem::OnlyOne;
        if (m_highlighted || m_checked)
            option.state |= QStyle::State_Selected;
        style->drawPrimitive(QStyle::PE_PanelItemViewItem, &option, painter);
        break;
    }
    case HeaderSection: {
        QStyleOptionHeader option;
        initOption(option);
        option.orientation = Qt::Horizontal;
        option.state |= QStyle::State_Raised;
        if (m_pressed)
            option.state |= QStyle::State_Sunken;
        switch (m_tabPosition) {
        case Beginning: option.position = QStyleOptionHeader::Beginning; break;
        case Middle: option.position = QStyleOptionHeader::Middle; break;
        case End: option.position = QStyleOptionHeader::End; break;
        case OnlyOneTab: option.position = QStyleOptionHeader::OnlyOneSection; break;
        }
        style->drawControl(QStyle::CE_HeaderSection, &option, painter);
        if (m_sortIndicator != Unsorted) {
            // QHeaderView と同じく、昇順を SortDown として渡す（Windows では上向きの矢印になる）。
            option.sortIndicator = m_sortIndicator == Ascending ? QStyleOptionHeader::SortDown
                                                                : QStyleOptionHeader::SortUp;
            option.rect = style->subElementRect(QStyle::SE_HeaderArrow, &option);
            style->drawPrimitive(QStyle::PE_IndicatorHeaderArrow, &option, painter);
        }
        break;
    }
    case ToolBar: {
        QStyleOptionToolBar option;
        initOption(option);
        option.toolBarArea = Qt::TopToolBarArea;
        option.positionOfLine = QStyleOptionToolBar::OnlyOne;
        option.positionWithinLine = QStyleOptionToolBar::OnlyOne;
        style->drawControl(QStyle::CE_ToolBar, &option, painter);
        break;
    }
    }
}
