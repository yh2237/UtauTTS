import QtQuick
import QtTest
import UtauTTS 1.0

Item {
    Main {
        id: appWindow
        injectedBackend: testBackend
        injectedLegalDocuments: []
        injectedAppName: "UtauTTS Qt Test"
        injectedRepositoryUrl: "https://example.invalid/"
        injectedSelfTest: true
        injectedIntonationLab: false
        injectedIntonationLabExamples: ""
        visible: true

        TestBackend {
            id: testBackend
        }

        Item {
            id: nonTextFocus
        }

        TestCase {
            name: "EditorContent"
            when: windowShown

            function init() {
                testBackend.busy = false;
                testBackend.removeUtteranceShortcut = "Delete";
                appWindow.batchExportActive = false;
                appWindow.playbackQueueActive = false;
                appWindow.clearPlayback();
                appWindow.utterancesModel.clear();
                appWindow.selectedIndex = 0;
                appWindow.addUtterance(false);
                appWindow.addUtterance(false);
                appWindow.requestActivate();
                tryCompare(appWindow, "active", true);
                waitForRendering(appWindow.contentItem);
                wait(1);
            }

            function editorWithText(text, cursor) {
                const editor = findChild(appWindow.contentItem, "utteranceText0");
                verify(editor !== null);
                editor.text = text;
                editor.forceActiveFocus();
                editor.deselect();
                editor.cursorPosition = cursor;
                tryCompare(editor, "activeFocus", true);
                compare(appWindow.selectedIndex, 0);
                return editor;
            }

            function test_deleteText_data() {
                return [
                    {
                        tag: "start",
                        text: "abcd",
                        cursor: 0,
                        expected: "bcd"
                    },
                    {
                        tag: "middle",
                        text: "abcd",
                        cursor: 2,
                        expected: "abd"
                    },
                    {
                        tag: "end",
                        text: "abcd",
                        cursor: 4,
                        expected: "abcd"
                    },
                    {
                        tag: "selection",
                        text: "abcd",
                        cursor: 0,
                        selectionStart: 1,
                        selectionEnd: 3,
                        expected: "ad"
                    },
                    {
                        tag: "all",
                        text: "abcd",
                        cursor: 0,
                        selectionStart: 0,
                        selectionEnd: 4,
                        expected: ""
                    },
                    {
                        tag: "lastCharacter",
                        text: "a",
                        cursor: 0,
                        expected: ""
                    },
                    {
                        tag: "space",
                        text: " ",
                        cursor: 0,
                        expected: ""
                    }
                ];
            }

            function test_deleteText(data) {
                const editor = editorWithText(data.text, data.cursor);
                if (data.selectionStart !== undefined)
                    editor.select(data.selectionStart, data.selectionEnd);
                const utteranceId = appWindow.current().utteranceId;
                keyClick(Qt.Key_Delete);
                compare(editor.text, data.expected);
                compare(appWindow.utterancesModel.get(0).content, data.expected);
                compare(appWindow.utterancesModel.count, 2);
                compare(appWindow.current().utteranceId, utteranceId);
            }

            function test_emptyDeleteRemovesUtterance() {
                editorWithText("", 0);
                const nextId = appWindow.utterancesModel.get(1).utteranceId;
                keyClick(Qt.Key_Delete);
                compare(appWindow.utterancesModel.count, 1);
                compare(appWindow.current().utteranceId, nextId);
            }

            function test_deleteLastCharacterBeforeUtterance() {
                const editor = editorWithText("a", 0);
                keyClick(Qt.Key_Delete);
                compare(editor.text, "");
                compare(appWindow.utterancesModel.count, 2);
                keyClick(Qt.Key_Delete);
                compare(appWindow.utterancesModel.count, 1);
            }

            function test_modifiedDeleteKeepsTextEditing() {
                const editor = editorWithText("alpha beta", 0);
                keyClick(Qt.Key_Delete, Qt.ControlModifier);
                verify(editor.text.length < "alpha beta".length);
                compare(appWindow.utterancesModel.count, 2);
            }

            function test_changedShortcutLeavesDeleteForText() {
                testBackend.removeUtteranceShortcut = "Ctrl+Shift+D";
                const editor = editorWithText("abc", 0);
                keyClick(Qt.Key_Delete);
                compare(editor.text, "bc");
                compare(appWindow.utterancesModel.count, 2);
                editor.text = "";
                keyClick(Qt.Key_Delete);
                compare(appWindow.utterancesModel.count, 2);
                keyClick(Qt.Key_D, Qt.ControlModifier | Qt.ShiftModifier);
                compare(appWindow.utterancesModel.count, 1);
            }

            function test_changedShortcutOutsideText() {
                testBackend.removeUtteranceShortcut = "Ctrl+Shift+D";
                editorWithText("abc", 0);
                nonTextFocus.forceActiveFocus();
                compare(appWindow.focusedUtteranceText(), null);
                keyClick(Qt.Key_D, Qt.ControlModifier | Qt.ShiftModifier);
                compare(appWindow.utterancesModel.count, 1);
            }

            function test_defaultShortcutOutsideText() {
                editorWithText("abc", 0);
                nonTextFocus.forceActiveFocus();
                compare(appWindow.focusedUtteranceText(), null);
                keyClick(Qt.Key_Delete);
                compare(appWindow.utterancesModel.count, 1);
            }

            function test_lastEmptyUtteranceCanBeDeleted() {
                editorWithText("", 0);
                appWindow.utterancesModel.remove(1);
                keyClick(Qt.Key_Delete);
                compare(appWindow.utterancesModel.count, 0);
                compare(appWindow.selectedIndex, 0);
            }

            function test_emptyDeleteDuringBatchExport() {
                editorWithText("", 0);
                appWindow.batchExportActive = true;
                keyClick(Qt.Key_Delete);
                compare(appWindow.utterancesModel.count, 2);
            }

            function test_emptyDeleteWhileBusy() {
                editorWithText("", 0);
                testBackend.busy = true;
                keyClick(Qt.Key_Delete);
                compare(appWindow.utterancesModel.count, 2);
            }

            function test_textDeleteWhileBusy() {
                const editor = editorWithText("abc", 0);
                testBackend.busy = true;
                keyClick(Qt.Key_Delete);
                compare(editor.text, "bc");
                compare(appWindow.utterancesModel.count, 2);
            }

            function test_emptyDeleteDuringPlaybackQueue() {
                editorWithText("", 0);
                appWindow.playbackQueueActive = true;
                keyClick(Qt.Key_Delete);
                compare(appWindow.utterancesModel.count, 2);
            }

            function test_pitchScrollBarsAlwaysVisibleInBothModes() {
                const basicBar = findChild(appWindow.contentItem, "basicPitchScrollBar");
                const extendedBar = findChild(appWindow.contentItem, "extendedPitchScrollBar");
                const tabs = findChild(appWindow.contentItem, "pitchModeTabs");
                const phonemeEditor = findChild(appWindow.contentItem, "phonemeEditor");
                verify(basicBar !== null);
                verify(extendedBar !== null);
                verify(tabs !== null);
                verify(phonemeEditor !== null);
                compare(tabs.currentIndex, 0);
                compare(basicBar.visible, true);
                tabs.currentIndex = 1;
                waitForRendering(appWindow.contentItem);
                compare(extendedBar.visible, true);
                compare(extendedBar.editor, phonemeEditor);
                tabs.currentIndex = 0;
            }

            function cleanupTestCase() {
                testBackend.busy = false;
                appWindow.playbackQueueActive = false;
                appWindow.projectDirty = false;
                appWindow.closeBypass = true;
                appWindow.close();
            }
        }
    }
}
