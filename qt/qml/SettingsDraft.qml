import QtQml

// SettingsDraftは、設定画面で編集中の値。保存するまでバックエンドへは書かない。
QtObject {
    property string defaultVoicebankId: ""
    property string defaultModelId: "frame-intonation-tcn-v12"
    property string defaultAliasPolicy: "auto"
    property string defaultTone: ""
    property string exportTextEncoding: "utf-8"
    property bool darkMode: false
    property string language: "auto"
    property string ffmpegPath: ""
    property string audioOutputDeviceId: ""
    property bool closeLogOnSuccess: true
    property bool updateCheckEnabled: true
    property bool preReleaseUpdateCheckEnabled: false
    property int previewCacheFileCount: 32
    property bool autoPreviewEnabled: true
    property bool extendedDetailsVisible: false
    property string synthesizeShortcut: "Ctrl+Enter"
    property string saveProjectShortcut: "Ctrl+S"
    property string reloadVoicebanksShortcut: "Ctrl+O"
    property string addUtteranceShortcut: "Ctrl+D"
    property string removeUtteranceShortcut: "Delete"
    property string undoShortcut: "Ctrl+Z"
    property string redoShortcut: "Ctrl+Y"
}
