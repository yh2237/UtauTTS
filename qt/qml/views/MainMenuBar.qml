import QtQuick
import QtQuick.Controls
import UtauTTS.Platform 1.0
import "../controls"

MenuBar {
    id: root
    required property var host
    visible: !root.host.mobileLayout
    height: visible ? implicitHeight : 0
    Menu {
        id: fileMenu
        title: root.host.translator.tr("menu.file")
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.file.open")
            enabled: !root.host.appBackend.busy && !root.host.batchExportActive
            onTriggered: root.host.openProject()
        }
        Menu {
            id: recentProjectsMenu
            title: root.host.translator.tr("menu.file.recent")

            Instantiator {
                model: root.host.appBackend.recentProjects
                delegate: GrayscaleMenuItem {
                    required property string modelData
                    text: root.host.recentProjectLabel(modelData)
                    enabled: !root.host.appBackend.busy && !root.host.batchExportActive
                    ToolTip.visible: hovered
                    ToolTip.text: modelData
                    ToolTip.delay: 500
                    onTriggered: root.host.loadRecentProject(modelData)
                }
                onObjectAdded: (index, object) => recentProjectsMenu.insertItem(index, object)
                onObjectRemoved: (index, object) => recentProjectsMenu.removeItem(object)
            }

            GrayscaleMenuItem {
                text: root.host.translator.tr("menu.file.recent.empty")
                enabled: false
                visible: root.host.appBackend.recentProjects.length === 0
            }
            MenuSeparator {}
            GrayscaleMenuItem {
                text: root.host.translator.tr("menu.file.recent.clear")
                enabled: root.host.appBackend.recentProjects.length > 0
                onTriggered: root.host.appBackend.clearRecentProjects()
            }
        }
        MenuSeparator {}
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.file.save")
            enabled: !root.host.appBackend.busy && !root.host.batchExportActive
            onTriggered: root.host.saveCurrentProject()
        }
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.file.saveAs")
            enabled: !root.host.appBackend.busy && !root.host.batchExportActive
            onTriggered: root.host.openProjectSaveDialog()
        }
        MenuSeparator {}
        Menu {
            id: exportMenu
            title: root.host.translator.tr("menu.file.export")
            GrayscaleMenuItem {
                text: root.host.translator.tr("menu.file.saveWav")
                enabled: utterances.count > 0 && !root.host.appBackend.busy && !root.host.batchExportActive && root.host.current().reading.length > 0
                onTriggered: root.host.saveCurrentAudio()
            }
            GrayscaleMenuItem {
                text: root.host.translator.tr("menu.file.saveAllWav")
                enabled: !root.host.appBackend.busy && !root.host.batchExportActive && root.host.hasPlayableTextFrom(0)
                onTriggered: root.host.openSaveAllDialog()
            }
            GrayscaleMenuItem {
                text: root.host.translator.tr("menu.file.exportUstx")
                enabled: utterances.count > 0 && !root.host.appBackend.busy && !root.host.batchExportActive
                onTriggered: root.host.openUstxExportDialog()
            }
            // exoの書き出しはネイティブのファイルダイアログがある環境だけ（末尾に足す）。
            MenuSeparator {
                visible: Platform.hasNativeFileDialog
                height: visible ? implicitHeight : 0
            }
            Instantiator {
                model: Platform.hasNativeFileDialog ? ["selected", "all"] : []
                delegate: GrayscaleMenuItem {
                    required property string modelData
                    text: modelData === "selected"
                          ? root.host.translator.tr("menu.file.exportExo")
                          : root.host.translator.tr("menu.file.exportAllExo")
                    enabled: modelData === "selected"
                             ? (utterances.count > 0 && !root.host.appBackend.busy && !root.host.batchExportActive && root.host.current().reading.length > 0)
                             : (!root.host.appBackend.busy && !root.host.batchExportActive && root.host.hasPlayableTextFrom(0))
                    onTriggered: root.host.openDragExportDialog(modelData === "selected")
                }
                onObjectAdded: (index, object) => exportMenu.addItem(object)
                onObjectRemoved: (index, object) => exportMenu.removeItem(object)
            }
        }
        Menu {
            id: voicebankMenu
            title: root.host.translator.tr("menu.file.voicebanks")
            GrayscaleMenuItem {
                text: root.host.translator.tr("menu.file.addVoicebank")
                enabled: !root.host.appBackend.busy && !root.host.batchExportActive
                onTriggered: root.host.addVoicebanks()
            }
            GrayscaleMenuItem {
                text: root.host.translator.tr("menu.file.reloadVoicebanks")
                enabled: !root.host.appBackend.busy
                onTriggered: root.host.reloadVoicebanks()
            }
            MenuSeparator {}
            GrayscaleMenuItem {
                text: root.host.translator.tr("plugins.title")
                enabled: !root.host.appBackend.busy && !root.host.batchExportActive
                onTriggered: root.host.menuDialogs.rendererPackages.open()
            }
            // 音源フォルダを開く操作はWeb以外（「音源を追加」の次）。
            Instantiator {
                model: Platform.isWeb ? 0 : 1
                delegate: GrayscaleMenuItem {
                    text: root.host.translator.tr("menu.file.openVoiceDirectory")
                    enabled: !root.host.appBackend.busy && !root.host.batchExportActive
                    onTriggered: root.host.appBackend.openVoiceDirectory()
                }
                onObjectAdded: (index, object) => voicebankMenu.insertItem(1, object)
                onObjectRemoved: (index, object) => voicebankMenu.removeItem(object)
            }
            // Classic UTAUのツールは外部ツールを使える環境だけ（末尾に足す）。
            MenuSeparator {
                visible: Platform.hasExternalTools
                height: visible ? implicitHeight : 0
            }
            Instantiator {
                model: Platform.hasExternalTools ? ["resampler", "wavtool", "reload"] : []
                delegate: GrayscaleMenuItem {
                    required property string modelData
                    text: modelData === "resampler"
                          ? root.host.translator.tr("menu.file.openResamplersDirectory")
                          : modelData === "wavtool"
                            ? root.host.translator.tr("menu.file.openWavtoolsDirectory")
                            : root.host.translator.tr("menu.file.reloadClassicTools")
                    enabled: modelData !== "reload" || !root.host.appBackend.busy
                    onTriggered: {
                        if (modelData === "reload")
                            root.host.appBackend.reloadClassicTools();
                        else
                            root.host.appBackend.openClassicToolDirectory(modelData);
                    }
                }
                onObjectAdded: (index, object) => voicebankMenu.addItem(object)
                onObjectRemoved: (index, object) => voicebankMenu.removeItem(object)
            }
        }
        MenuSeparator {}
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.file.quit")
            onTriggered: Qt.quit()
        }
    }
    Menu {
        title: root.host.translator.tr("menu.edit")
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.edit.undo")
            enabled: root.host.canUndo && !root.host.appBackend.busy && !root.host.batchExportActive
                     && !root.host.playbackQueueActive
            onTriggered: root.host.undo()
        }
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.edit.redo")
            enabled: root.host.canRedo && !root.host.appBackend.busy && !root.host.batchExportActive
                     && !root.host.playbackQueueActive
            onTriggered: root.host.redo()
        }
    }
    Menu {
        title: root.host.translator.tr("menu.playback")
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.playback.current")
            enabled: utterances.count > 0 && !root.host.appBackend.busy && !root.host.batchExportActive
                     && !root.host.playbackQueueActive && root.host.current().reading.length > 0
            onTriggered: root.host.synthesizeCurrent()
        }
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.playback.all")
            enabled: !root.host.appBackend.busy && !root.host.batchExportActive && !root.host.playbackQueueActive
                     && root.host.hasPlayableTextFrom(0)
            onTriggered: root.host.startPlaybackQueue(0)
        }
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.playback.fromSelected")
            enabled: !root.host.appBackend.busy && !root.host.batchExportActive && !root.host.playbackQueueActive
                     && root.host.hasPlayableTextFrom(root.host.selectedIndex)
            onTriggered: root.host.startPlaybackQueue(root.host.selectedIndex)
        }
        MenuSeparator {}
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.playback.replay")
            enabled: !root.host.appBackend.busy && !root.host.batchExportActive && !root.host.playbackQueueActive
                     && root.host.hasCachedAudio()
            onTriggered: root.host.replayCachedAudio()
        }
    }
    Menu {
        title: root.host.translator.tr("menu.settings")
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.settings.settings")
            onTriggered: root.host.openSettings()
        }
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.settings.dictionary")
            onTriggered: root.host.openDictionarySettings()
        }
    }
    Menu {
        title: root.host.translator.tr("menu.help")
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.help.about")
            onTriggered: {
                if (!root.host.appBackend.showNativeAboutDialog())
                    root.host.menuDialogs.about.open();
            }
        }
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.help.repository")
            onTriggered: Qt.openUrlExternally(root.host.repositoryUrl)
        }
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.help.license")
            onTriggered: root.host.openLicense()
        }
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.help.voicebankDetails")
            enabled: root.host.appBackend.voicebanks.length > 0
            onTriggered: root.host.showVoicebankDetails()
        }
        GrayscaleMenuItem {
            text: root.host.translator.tr("menu.help.exportDiagnostics")
            onTriggered: {
                const destination = root.host.appBackend.defaultSaveFile("utautts-diagnostics.json");
                if (Platform.hasNativeFileDialog) {
                    diagnosticSaveDialog.currentFile = destination;
                    diagnosticSaveDialog.open();
                } else {
                    root.host.exportDiagnosticsTo(destination);
                }
            }
        }
    }
}
