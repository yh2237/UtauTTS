import QtQuick
import QtQuick.Controls
import QtQuick.Dialogs
import QtQuick.Layouts

Item {
    id: root
    required property var host
    visible: false
    property alias saveDialog : saveDialog
    property alias saveAllDialog : saveAllDialog
    property alias dragSaveDialog : dragSaveDialog
    property alias voicebankAddDialog : voicebankAddDialog
    property alias frameRateDialog : frameRateDialog
    property alias projectSaveDialog : projectSaveDialog
    property alias ustxExportFileDialog : ustxExportFileDialog
    property alias rendererPackagesDialog : rendererPackagesDialog
    property alias projectOpenDialog : projectOpenDialog
    property alias diagnosticSaveDialog : diagnosticSaveDialog
    property alias diagnosticResultDialog : diagnosticResultDialog
    property alias ustxExportMessageDialog : ustxExportMessageDialog
    property alias closeWarningDialog : closeWarningDialog
    property alias shortcutConflictDialog : shortcutConflictDialog
    property alias projectLoadErrorDialog : projectLoadErrorDialog
    property alias aboutDialog : aboutDialog
    property alias updateDialog : updateDialog
    property alias updateProgressDialog : updateProgressDialog
    property alias metadataReloadDialog : metadataReloadDialog

    FileDialog {
        id: saveDialog
        fileMode: FileDialog.SaveFile
        nameFilters: [host.translator.tr("main.wavFilter")]
        defaultSuffix: "wav"
        onAccepted: host.appBackend.savePreview(selectedFile)
    }

    FolderDialog {
        id: saveAllDialog
        onAccepted: host.startBatchExport(selectedFolder)
    }

    FolderDialog {
        id: dragSaveDialog
        onAccepted: host.startDragExport(selectedFolder)
    }

    FileDialog {
        id: voicebankAddDialog
        title: host.translator.tr("menu.file.addVoicebank")
        fileMode: FileDialog.OpenFiles
        nameFilters: [host.translator.tr("main.zipFilter")]
        onAccepted: host.appBackend.installVoicebankArchives(selectedFiles)
    }

    Dialog {
        id: frameRateDialog
        title: host.translator.tr("main.exoFrameRateTitle")
        modal: true
        width: Math.min(host.width - 40, 400)
        anchors.centerIn: Overlay.overlay
        closePolicy: Popup.CloseOnEscape
        standardButtons: Dialog.Ok | Dialog.Cancel
        onAccepted: {
            host.dragExportFrameRate = frameRateSpin.value;
            dragSaveDialog.open();
        }

        contentItem: ColumnLayout {
            spacing: 12

            RowLayout {
                Layout.fillWidth: true
                spacing: 8

                Label {
                    text: host.translator.tr("main.frameRate")
                }
                SpinBox {
                    id: frameRateSpin
                    Layout.preferredWidth: 120
                    from: 1
                    to: 240
                    stepSize: 1
                    value: 60
                    editable: true
                }
                Label {
                    text: host.translator.tr("main.fps")
                    color: host.mutedText
                }
                Item {
                    Layout.fillWidth: true
                }
            }
        }
    }

    readonly property var dragTargetWindow: dragTargetWindowLoader.item

    FileDialog {
        id: projectSaveDialog
        fileMode: FileDialog.SaveFile
        nameFilters: [host.translator.tr("main.projectFilter")]
        defaultSuffix: "utautts"
        onAccepted: host.saveProjectTo(selectedFile)
        onRejected: host.closeAfterProjectSave = false
    }

    FileDialog {
        id: ustxExportFileDialog
        fileMode: FileDialog.SaveFile
        nameFilters: [host.translator.tr("main.ustxFilter")]
        defaultSuffix: "ustx"
        onAccepted: host.exportUstxTo(selectedFile)
    }

    Dialog {
        id: rendererPackagesDialog
        title: host.translator.tr("plugins.title")
        anchors.centerIn: parent
        width: Math.min(620, host.width - 40)
        height: Math.min(540, host.height - 40)
        modal: true
        standardButtons: Dialog.Close
        contentItem: ColumnLayout {
            ScrollView {
                Layout.fillWidth: true
                Layout.fillHeight: true
                Column {
                    width: parent.width
                    spacing: 10
                    Repeater {
                        model: host.appBackend.renderers
                        delegate: Label {
                            required property var modelData
                            width: parent.width
                            wrapMode: Text.Wrap
                            text: modelData.display_name + "  " + (modelData.version || "")
                        }
                    }
                    Label {
                        width: parent.width
                        visible: host.appBackend.pluginProblems.length > 0
                        wrapMode: Text.Wrap
                        text: host.translator.tr("plugins.disabled") + "\n" + host.appBackend.pluginProblems.join("\n\n")
                    }
                }
            }
            Label {
                Layout.fillWidth: true
                wrapMode: Text.Wrap
                text: host.appBackend.error
                visible: text.length > 0
            }
        }
    }

    FileDialog {
        id: projectOpenDialog
        fileMode: FileDialog.OpenFile
        nameFilters: [host.translator.tr("main.projectFilter")]
        onAccepted: host.loadProjectFrom(selectedFile)
    }

    FileDialog {
        id: diagnosticSaveDialog
        fileMode: FileDialog.SaveFile
        nameFilters: [host.translator.tr("diagnostics.filter")]
        defaultSuffix: "json"
        onAccepted: host.exportDiagnosticsTo(selectedFile)
    }

    MessageDialog {
        id: diagnosticResultDialog
        buttons: MessageDialog.Ok
    }

    MessageDialog {
        id: ustxExportMessageDialog
        title: host.translator.tr("main.ustxExportTitle")
        buttons: MessageDialog.Ok
    }

    Dialog {
        id: closeWarningDialog
        title: host.translator.tr("main.closeConfirmTitle")
        modal: true
        width: Math.min(host.width - 40, 460)
        anchors.centerIn: Overlay.overlay
        closePolicy: Popup.NoAutoClose

        contentItem: ColumnLayout {
            spacing: 12

            Label {
                Layout.fillWidth: true
                text: host.appBackend.busy || host.batchExportActive
                      ? host.translator.tr("main.closeWhileBusy")
                      : host.translator.tr("main.closeUnsaved")
                wrapMode: Text.WordWrap
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: 8

                Item {
                    Layout.fillWidth: true
                }

                Button {
                    text: host.translator.tr("main.cancel")
                    onClicked: closeWarningDialog.close()
                }

                Button {
                    text: host.translator.tr("main.saveAndQuit")
                    enabled: host.projectDirty && !host.appBackend.busy && !host.batchExportActive
                    onClicked: {
                        closeWarningDialog.close();
                        host.closeAfterProjectSave = true;
                        host.saveCurrentProject();
                    }
                }

                Button {
                    text: host.translator.tr("main.quitWithoutSaving")
                    onClicked: {
                        closeWarningDialog.close();
                        host.quitWithoutWarning();
                    }
                }
            }
        }
    }

    MessageDialog {
        id: shortcutConflictDialog
        title: host.translator.tr("main.shortcutConflictTitle")
        text: host.translator.tr("main.shortcutConflictText")
        buttons: MessageDialog.Ok
    }

    MessageDialog {
        id: projectLoadErrorDialog
        title: host.translator.tr("main.projectOpenErrorTitle")
        buttons: MessageDialog.Ok
    }

    Dialog {
        id: aboutDialog
        title: host.translator.tr("main.aboutTitle")
        modal: true
        anchors.centerIn: Overlay.overlay
        width: Math.min(host.width - 40, 440)
        closePolicy: Popup.CloseOnEscape
        standardButtons: Dialog.Ok

        contentItem: ColumnLayout {
            spacing: 8
            Label {
                Layout.fillWidth: true
                wrapMode: Text.WordWrap
                text: host.translator.tr("main.aboutText", Qt.application.version)
            }
            Label {
                Layout.fillWidth: true
                wrapMode: Text.WordWrap
                color: host.mutedText
                text: host.translator.tr("main.aboutInformative")
            }
        }
    }

    Dialog {
        id: updateDialog
        title: host.translator.tr("update.title")
        modal: true
        width: Math.min(host.width - 40, 480)
        anchors.centerIn: Overlay.overlay
        closePolicy: Popup.CloseOnEscape
        standardButtons: Dialog.NoButton

        contentItem: ColumnLayout {
            spacing: 12

            Label {
                Layout.fillWidth: true
                text: host.translator.tr("update.message", host.updateAvailableVersion)
                wrapMode: Text.WordWrap
            }

            Label {
                Layout.fillWidth: true
                visible: host.updateAvailablePreRelease
                text: host.translator.tr("update.preReleaseNotice")
                wrapMode: Text.WordWrap
                color: host.mutedText
                font.pixelSize: 11
            }

            ScrollView {
                Layout.fillWidth: true
                Layout.preferredHeight: 150
                clip: true
                TextArea {
                    readOnly: true
                    wrapMode: Text.WordWrap
                    text: host.updateReleaseNotes + (host.updateReleaseNotes.length ? "\n\n" : "") + host.updateReleaseUrl
                }
            }

            Label {
                Layout.fillWidth: true
                text: host.translator.tr("update.preserveNote")
                wrapMode: Text.WordWrap
                color: host.mutedText
                font.pixelSize: 11
            }

            CheckBox {
                id: suppressUpdateVersionCheckBox
                Layout.fillWidth: true
                text: host.translator.tr("update.suppressVersion")
                checked: host.updateSuppressVersion
                onToggled: {
                    host.updateSuppressVersion = checked;
                    host.appBackend.setSuppressedUpdateVersion(checked ? host.updateAvailableVersion : "");
                }
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: 8

                Button {
                    text: host.translator.tr("update.button")
                    highlighted: true
                    onClicked: host.performUpdate()
                }
                Button {
                    text: host.translator.tr("update.openRelease")
                    onClicked: Qt.openUrlExternally(host.updateReleaseUrl)
                }
                Button {
                    text: host.translator.tr("update.later")
                    onClicked: updateDialog.close()
                }
            }
        }
    }

    Dialog {
        id: updateProgressDialog
        title: host.translator.tr("update.title")
        modal: true
        width: Math.min(host.width - 40, 440)
        anchors.centerIn: Overlay.overlay
        closePolicy: Popup.NoAutoClose
        standardButtons: Dialog.NoButton

        contentItem: ColumnLayout {
            spacing: 12

            Label {
                Layout.fillWidth: true
                text: host.translator.tr("update.downloading")
                wrapMode: Text.WordWrap
            }

            ProgressBar {
                Layout.fillWidth: true
                from: 0
                to: 1
                value: host.updateDownloadTotal > 0 ? host.updateDownloadReceived / host.updateDownloadTotal : 0
                indeterminate: host.updateDownloadTotal <= 0
            }

            Label {
                Layout.fillWidth: true
                text: host.updateDownloadTotal > 0
                    ? Math.floor(host.updateDownloadReceived / 1048576) + " / "
                      + Math.floor(host.updateDownloadTotal / 1048576) + " MB"
                    : ""
                color: host.mutedText
                font.pixelSize: 11
                horizontalAlignment: Text.AlignHCenter
            }

            Button {
                Layout.alignment: Qt.AlignHCenter
                text: host.translator.tr("common.cancel")
                onClicked: {
                    host.appBackend.cancelUpdateDownload();
                    updateProgressDialog.close();
                }
            }
        }
    }

    Dialog {
        id: metadataReloadDialog
        title: host.translator.tr("metadata.loading." + host.metadataReloadStage)
        modal: true
        width: Math.min(host.width - 40, 440)
        anchors.centerIn: Overlay.overlay
        closePolicy: Popup.NoAutoClose
        standardButtons: Dialog.NoButton

        contentItem: ColumnLayout {
            spacing: 12

            ProgressBar {
                Layout.fillWidth: true
                indeterminate: true
            }
        }
    }

}
