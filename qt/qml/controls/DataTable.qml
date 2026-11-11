pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import UtauTTSStyle

// 編集できる表では、ダブルクリック・F2・Enter・選択中のセルのクリックで書き換える。
// 並べ替えと絞り込みは表示の順番だけを変え、行の番号（selectedRow など）は元のモデルの番号のまま。
ListFrame {
    id: root

    property var columns: []
    property var model: []
    property var cellValue: (row, column) => ""
    property var sortValue: (row, column) => root.cellValue(row, column)
    property var filter: null
    property var placeholders: []
    property bool editable: false
    property bool compact: false
    property int selectedRow: -1
    property int editingRow: -1
    property int editingColumn: -1
    property int sortColumn: -1
    property bool sortDescending: false
    property string emptyText: ""
    property var order: []
    readonly property bool listModel: !!root.model && typeof root.model.get === "function"
    property int revision: 0
    signal cellEdited(int row, int column, string text)
    signal deleteRequested()

    function rowCount() {
        if (!root.model)
            return 0;
        return root.listModel ? root.model.count : root.model.length;
    }

    function rowAt(row) {
        return root.listModel ? root.model.get(row) : root.model[row];
    }

    function rebuildOrder() {
        const rows = [];
        for (let row = 0; row < root.rowCount(); ++row) {
            if (!root.filter || root.filter(root.rowAt(row)))
                rows.push(row);
        }
        if (root.sortColumn >= 0) {
            const keys = rows.map(row => root.sortValue(root.rowAt(row), root.sortColumn));
            const positions = rows.map((row, position) => position);
            positions.sort((a, b) => {
                const left = keys[a];
                const right = keys[b];
                const compared = typeof left === "number" && typeof right === "number"
                        ? left - right
                        : String(left ?? "").localeCompare(String(right ?? ""), undefined, {numeric: true});
                return (root.sortDescending ? -compared : compared) || a - b;
            });
            root.order = positions.map(position => rows[position]);
        } else {
            root.order = rows;
        }
        if (root.selectedRow >= root.rowCount())
            root.selectedRow = -1;
    }

    // 並べ替えや絞り込みで行が動くと一覧が作り直されるので、スクロール位置を戻す。
    function refreshOrder() {
        const contentY = list.contentY;
        root.rebuildOrder();
        list.contentY = Math.max(0, Math.min(contentY, list.contentHeight - list.height));
    }

    function sortBy(column) {
        if (root.sortColumn === column)
            root.sortDescending = !root.sortDescending;
        else {
            root.sortColumn = column;
            root.sortDescending = false;
        }
        root.refreshOrder();
    }

    function viewIndexOf(row) {
        return root.order.indexOf(row);
    }

    function editCell(row, column) {
        const viewIndex = root.viewIndexOf(row);
        if (!root.editable || viewIndex < 0)
            return;
        root.selectedRow = row;
        list.positionViewAtIndex(viewIndex, ListView.Contain);
        root.editingRow = row;
        root.editingColumn = column;
    }

    function finishEditing() {
        root.editingRow = -1;
        root.editingColumn = -1;
        list.forceActiveFocus();
    }

    function moveSelection(step) {
        const viewIndex = root.viewIndexOf(root.selectedRow);
        const next = viewIndex < 0 ? 0 : viewIndex + step;
        if (next < 0 || next >= root.order.length)
            return;
        root.selectedRow = root.order[next];
        list.positionViewAtIndex(next, ListView.Contain);
    }

    onModelChanged: rebuildOrder()
    onFilterChanged: refreshOrder()
    Component.onCompleted: rebuildOrder()

    Connections {
        target: root.listModel ? root.model : null
        ignoreUnknownSignals: true
        function onCountChanged() { root.refreshOrder(); }
        function onDataChanged() {
            root.revision += 1;
            if (root.sortColumn >= 0 || root.filter)
                root.refreshOrder();
        }
    }

    Item {
        id: body
        anchors.fill: parent

        ColumnLayout {
            anchors.fill: parent
            anchors.rightMargin: scrollBar.visible ? scrollBar.width : 0
            spacing: 0

            RowLayout {
                Layout.fillWidth: true
                spacing: 0

                Repeater {
                    id: header
                    model: root.columns
                    delegate: ColumnHeader {
                        required property int index
                        required property var modelData
                        Layout.fillWidth: true
                        Layout.minimumWidth: 0
                        Layout.preferredWidth: modelData.width
                        position: root.columns.length === 1 ? StyleItem.OnlyOneTab
                                : index === 0 ? StyleItem.Beginning
                                : index === root.columns.length - 1 ? StyleItem.End : StyleItem.Middle
                        text: modelData.title
                        clickable: true
                        sortIndicator: root.sortColumn !== index ? StyleItem.Unsorted
                                     : root.sortDescending ? StyleItem.Descending : StyleItem.Ascending
                        onClicked: root.sortBy(index)
                    }
                }
            }

            ListView {
                id: list
                readonly property real textPadding: header.count ? header.itemAt(0).textPadding : 0
                Layout.fillWidth: true
                Layout.fillHeight: true
                clip: true
                focus: true
                model: root.order
                ScrollBar.vertical: ScrollBar {
                    id: scrollBar
                    parent: body
                    x: body.width - width
                    y: 0
                    height: body.height
                    policy: root.compact ? ScrollBar.AlwaysOff : ScrollBar.AlwaysOn
                }

                Keys.onPressed: event => {
                    if (root.editingRow >= 0)
                        return;
                    if (event.key === Qt.Key_Delete) {
                        root.deleteRequested();
                    } else if (event.key === Qt.Key_Up) {
                        root.moveSelection(-1);
                    } else if (event.key === Qt.Key_Down) {
                        root.moveSelection(1);
                    } else if ((event.key === Qt.Key_F2 || event.key === Qt.Key_Return
                                || event.key === Qt.Key_Enter) && root.selectedRow >= 0) {
                        root.editCell(root.selectedRow, 0);
                    } else {
                        return;
                    }
                    event.accepted = true;
                }

                delegate: ItemDelegate {
                    id: row
                    required property var modelData
                    readonly property int sourceRow: modelData
                    width: ListView.view.width
                    height: root.compact ? Math.max(implicitHeight, Theme.touchTarget) : implicitHeight
                    leftPadding: 0
                    rightPadding: 0
                    highlighted: sourceRow === root.selectedRow

                    function columnAt(x) {
                        for (let column = cells.count - 1; column >= 0; --column) {
                            const cell = cells.itemAt(column);
                            if (cell && x >= cell.x)
                                return column;
                        }
                        return 0;
                    }

                    onClicked: {
                        if (root.editable && row.highlighted) {
                            root.editCell(row.sourceRow, columnAt(pressX));
                            return;
                        }
                        root.selectedRow = row.sourceRow;
                        list.forceActiveFocus();
                    }
                    onDoubleClicked: root.editCell(row.sourceRow, columnAt(pressX))

                    contentItem: RowLayout {
                        spacing: 0

                        Repeater {
                            id: cells
                            model: root.columns.length
                            delegate: Item {
                                id: cell
                                required property int index
                                readonly property string value: root.revision >= 0
                                        ? String(root.cellValue(root.rowAt(row.sourceRow), index) ?? "") : ""
                                readonly property bool editing: root.editingRow === row.sourceRow
                                                                && root.editingColumn === index
                                Layout.fillWidth: true
                                Layout.fillHeight: true
                                Layout.minimumWidth: 0
                                Layout.preferredWidth: root.columns[index].width
                                implicitHeight: label.implicitHeight

                                Label {
                                    id: label
                                    anchors.fill: parent
                                    leftPadding: list.textPadding
                                    rightPadding: list.textPadding
                                    verticalAlignment: Text.AlignVCenter
                                    visible: !cell.editing
                                    text: cell.value.length ? cell.value : (root.placeholders[cell.index] ?? "")
                                    elide: Text.ElideRight
                                    color: !cell.value.length ? row.palette.placeholderText
                                            : row.highlighted ? row.palette.highlightedText : row.palette.text
                                }

                                Loader {
                                    parent: row
                                    x: cell.x + row.leftPadding
                                    width: cell.width
                                    height: row.height
                                    active: cell.editing
                                    sourceComponent: TextField {
                                        id: editor
                                        property bool committed: false
                                        readonly property int rowIndex: row.sourceRow
                                        readonly property int columnIndex: cell.index

                                        function commit() {
                                            if (editor.committed)
                                                return;
                                            editor.committed = true;
                                            root.cellEdited(editor.rowIndex, editor.columnIndex, editor.text);
                                        }

                                        leftPadding: list.textPadding
                                        rightPadding: list.textPadding
                                        topPadding: 0
                                        bottomPadding: 0
                                        verticalAlignment: TextInput.AlignVCenter
                                        background: Rectangle {
                                            color: editor.palette.base
                                        }
                                        text: cell.value
                                        placeholderText: root.placeholders[cell.index] ?? ""
                                        selectByMouse: true
                                        Component.onCompleted: {
                                            forceActiveFocus();
                                            selectAll();
                                        }
                                        Component.onDestruction: commit()
                                        onAccepted: {
                                            commit();
                                            root.finishEditing();
                                        }
                                        onActiveFocusChanged: {
                                            if (activeFocus)
                                                return;
                                            commit();
                                            if (root.editingRow === editor.rowIndex
                                                    && root.editingColumn === editor.columnIndex)
                                                root.finishEditing();
                                        }
                                        Keys.onEscapePressed: {
                                            editor.committed = true;
                                            root.finishEditing();
                                        }
                                        Keys.onTabPressed: {
                                            const next = editor.columnIndex + 1;
                                            const target = editor.rowIndex;
                                            commit();
                                            if (next < root.columns.length)
                                                root.editCell(target, next);
                                            else
                                                root.finishEditing();
                                        }
                                    }
                                }
                            }
                        }
                    }
                }

                Label {
                    x: list.textPadding
                    y: Theme.spacingSmall
                    width: list.width - 2 * x
                    visible: list.count === 0 && root.emptyText.length > 0
                    text: root.emptyText
                    wrapMode: Text.WordWrap
                }
            }
        }
    }
}
