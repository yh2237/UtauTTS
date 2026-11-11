#pragma once

#include <QFont>
#include <QMarginsF>
#include <QQuickPaintedItem>
#include <QtQml/qqmlregistration.h>

class QStyleOption;
class QStyleOptionSlider;

class StyleItem : public QQuickPaintedItem {
    Q_OBJECT
    QML_ELEMENT
    Q_PROPERTY(Element element READ element WRITE setElement NOTIFY metricsChanged)
    Q_PROPERTY(bool hovered MEMBER m_hovered WRITE setHovered NOTIFY stateChanged)
    Q_PROPERTY(bool pressed MEMBER m_pressed WRITE setPressed NOTIFY stateChanged)
    Q_PROPERTY(bool checked MEMBER m_checked WRITE setChecked NOTIFY stateChanged)
    Q_PROPERTY(bool partiallyChecked MEMBER m_partiallyChecked WRITE setPartiallyChecked NOTIFY stateChanged)
    Q_PROPERTY(bool focused MEMBER m_focused WRITE setFocused NOTIFY stateChanged)
    Q_PROPERTY(bool flat MEMBER m_flat WRITE setFlat NOTIFY metricsChanged)
    Q_PROPERTY(bool highlighted MEMBER m_highlighted WRITE setHighlighted NOTIFY stateChanged)
    Q_PROPERTY(bool editable MEMBER m_editable WRITE setEditable NOTIFY metricsChanged)
    Q_PROPERTY(bool checkable MEMBER m_checkable WRITE setCheckable NOTIFY stateChanged)
    Q_PROPERTY(bool hasSubmenu MEMBER m_hasSubmenu WRITE setHasSubmenu NOTIFY stateChanged)
    Q_PROPERTY(bool upEnabled MEMBER m_upEnabled WRITE setUpEnabled NOTIFY stateChanged)
    Q_PROPERTY(bool downEnabled MEMBER m_downEnabled WRITE setDownEnabled NOTIFY stateChanged)
    Q_PROPERTY(Qt::Orientation orientation MEMBER m_orientation WRITE setOrientation NOTIFY metricsChanged)
    Q_PROPERTY(qreal value MEMBER m_value WRITE setValue NOTIFY stateChanged)
    Q_PROPERTY(qreal size MEMBER m_size WRITE setBarSize NOTIFY stateChanged)
    Q_PROPERTY(TabPosition tabPosition MEMBER m_tabPosition WRITE setTabPosition NOTIFY stateChanged)
    Q_PROPERTY(SortIndicator sortIndicator MEMBER m_sortIndicator WRITE setSortIndicator NOTIFY stateChanged)
    Q_PROPERTY(SubControl hoveredControl MEMBER m_hoveredControl WRITE setHoveredControl NOTIFY stateChanged)
    Q_PROPERTY(SubControl pressedControl MEMBER m_pressedControl WRITE setPressedControl NOTIFY stateChanged)
    Q_PROPERTY(QFont font MEMBER m_font WRITE setFont NOTIFY metricsChanged)

    Q_PROPERTY(qreal paddingLeft READ paddingLeft NOTIFY metricsChanged)
    Q_PROPERTY(qreal paddingTop READ paddingTop NOTIFY metricsChanged)
    Q_PROPERTY(qreal paddingRight READ paddingRight NOTIFY metricsChanged)
    Q_PROPERTY(qreal paddingBottom READ paddingBottom NOTIFY metricsChanged)
    Q_PROPERTY(qreal minimumWidth READ minimumWidth NOTIFY metricsChanged)
    Q_PROPERTY(qreal spacing READ spacing NOTIFY metricsChanged)
    Q_PROPERTY(qreal extent READ extent NOTIFY metricsChanged)
    Q_PROPERTY(qreal handleLength READ handleLength NOTIFY metricsChanged)

    Q_PROPERTY(qreal lineButtonLength READ lineButtonLength NOTIFY geometryChanged)
    Q_PROPERTY(QRectF upRect READ upRect NOTIFY geometryChanged)
    Q_PROPERTY(QRectF downRect READ downRect NOTIFY geometryChanged)
    Q_PROPERTY(QRectF handleRect READ handleRect NOTIFY geometryChanged)

public:
    enum Element {
        Button,
        ToolButton,
        CheckBox,
        RadioButton,
        ComboBox,
        SpinBox,
        TextField,
        Frame,
        Slider,
        ProgressBar,
        ScrollBar,
        Tab,
        TabFrame,
        MenuBar,
        MenuBarItem,
        MenuPanel,
        MenuItem,
        MenuSeparator,
        ToolTip,
        Splitter,
        ItemView,
        ToolBar,
        HeaderSection,
    };
    Q_ENUM(Element)

    enum SubControl { None, SubLine, AddLine, SubPage, AddPage, Handle, Groove, Up, Down, Arrow, EditField };
    Q_ENUM(SubControl)

    enum TabPosition { Beginning, Middle, End, OnlyOneTab };
    Q_ENUM(TabPosition)

    enum SortIndicator { Unsorted, Ascending, Descending };
    Q_ENUM(SortIndicator)

    enum Metric { TabBarBaseOverlap, DefaultFrameWidth, WindowMargin, LayoutSpacing };
    Q_ENUM(Metric)

    explicit StyleItem(QQuickItem *parent = nullptr);

    Element element() const { return m_element; }
    void setElement(Element value) { assignMetric(m_element, value); }
    void setHovered(bool value) { assign(m_hovered, value); }
    void setPressed(bool value) { assign(m_pressed, value); }
    void setChecked(bool value) { assign(m_checked, value); }
    void setPartiallyChecked(bool value) { assign(m_partiallyChecked, value); }
    void setFocused(bool value) { assign(m_focused, value); }
    void setFlat(bool value) { assignMetric(m_flat, value); }
    void setHighlighted(bool value) { assign(m_highlighted, value); }
    void setEditable(bool value) { assignMetric(m_editable, value); }
    void setCheckable(bool value) { assign(m_checkable, value); }
    void setHasSubmenu(bool value) { assign(m_hasSubmenu, value); }
    void setUpEnabled(bool value) { assign(m_upEnabled, value); }
    void setDownEnabled(bool value) { assign(m_downEnabled, value); }
    void setOrientation(Qt::Orientation value) { assignMetric(m_orientation, value); }
    void setValue(qreal value) { assign(m_value, value); }
    void setBarSize(qreal value) { assign(m_size, value); }
    void setTabPosition(TabPosition value) { assign(m_tabPosition, value); }
    void setSortIndicator(SortIndicator value) { assign(m_sortIndicator, value); }
    void setHoveredControl(SubControl value) { assign(m_hoveredControl, value); }
    void setPressedControl(SubControl value) { assign(m_pressedControl, value); }
    void setFont(const QFont &value) { assignMetric(m_font, value); }

    qreal paddingLeft() const { return m_padding.left(); }
    qreal paddingTop() const { return m_padding.top(); }
    qreal paddingRight() const { return m_padding.right(); }
    qreal paddingBottom() const { return m_padding.bottom(); }
    qreal minimumWidth() const { return m_minimumWidth; }
    qreal spacing() const { return m_spacing; }
    qreal extent() const { return m_extent; }
    qreal handleLength() const { return m_handleLength; }

    qreal lineButtonLength() const;
    QRectF upRect() const { return subControlRect(Up); }
    QRectF downRect() const { return subControlRect(Down); }
    QRectF handleRect() const { return subControlRect(Handle); }

    Q_INVOKABLE int pixelMetric(Metric metric) const;
    Q_INVOKABLE SubControl hitTest(qreal x, qreal y) const;

    void paint(QPainter *painter) override;

signals:
    void stateChanged();
    void metricsChanged();
    void geometryChanged();

protected:
    void geometryChange(const QRectF &newGeometry, const QRectF &oldGeometry) override;

private:
    template <typename T>
    void assign(T &field, const T &value) {
        if (field == value)
            return;
        field = value;
        emit stateChanged();
    }

    template <typename T>
    void assignMetric(T &field, const T &value) {
        if (field == value)
            return;
        field = value;
        updateMetrics();
    }

    void updateMetrics();
    QRectF subControlRect(SubControl control) const;
    void initOption(QStyleOption &option) const;
    QStyleOptionSlider sliderOption() const;

    Element m_element = Button;
    bool m_hovered = false;
    bool m_pressed = false;
    bool m_checked = false;
    bool m_partiallyChecked = false;
    bool m_focused = false;
    bool m_flat = false;
    bool m_highlighted = false;
    bool m_editable = false;
    bool m_checkable = false;
    bool m_hasSubmenu = false;
    bool m_upEnabled = true;
    bool m_downEnabled = true;
    Qt::Orientation m_orientation = Qt::Horizontal;
    qreal m_value = 0;
    qreal m_size = 1;
    TabPosition m_tabPosition = OnlyOneTab;
    SortIndicator m_sortIndicator = Unsorted;
    SubControl m_hoveredControl = None;
    SubControl m_pressedControl = None;
    QFont m_font;

    QMarginsF m_padding;
    qreal m_minimumWidth = 0;
    qreal m_spacing = 0;
    qreal m_extent = 0;
    qreal m_handleLength = 0;
};
