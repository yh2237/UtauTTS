import QtQuick

QtObject {
    property bool busy: false
    property bool darkMode: false
    property string language: "ja"
    property bool onboardingCompleted: true
    property bool updateCheckEnabled: false
    property bool autoPreviewEnabled: false
    property bool extendedDetailsVisible: false
    property string audioOutputDeviceId: ""
    property string error: ""
    property var voicebanks: []
    property var models: []
    property var retiredModels: ({})
    property var settingDefinitions: []
    property var synthesisSettings: ({})
    property var referenceProviders: []
    property var referenceIndexes: []
    property bool referenceIndexBuilding: false
    property real referenceIndexProgress: 0
    property string defaultVoicebankId: ""
    property string defaultModelId: "none"
    property string defaultAliasPolicy: "auto"
    property string defaultTone: ""
    property int defaultMoraDuration: 120
    property int defaultPauseDuration: 180
    property int defaultLeadingPreutterance: 0
    property real defaultIntonationStrength: 1
    property string synthesizeShortcut: "Ctrl+Return"
    property string saveProjectShortcut: "Ctrl+S"
    property string reloadVoicebanksShortcut: "Ctrl+R"
    property string addUtteranceShortcut: "Ctrl+Shift+N"
    property string removeUtteranceShortcut: "Delete"
    property string undoShortcut: "Ctrl+Z"
    property string redoShortcut: "Ctrl+Y"
    property var recentProjects: []

    signal projectPicked(var project)
    signal ustxExportFinished(bool success, string detail)
    signal updateDownloadProgress(real bytesReceived, real bytesTotal)
    signal updateDownloadFinished(bool success, string localZip)
    signal metadataReloadStarted()
    signal metadataReloadStageChanged(string stage)
    signal referenceIndexChanged()
    signal metadataChanged()
    signal analysisChanged()
    signal prosodyChanged()
    signal previewReady()
    signal synthesisCanceled()
    signal audioOutputSettingsChanged()

    function languageCodes() { return ["ja"]; }
    function languageDisplayName(code) { return code; }
    function resolvedLanguage() { return "ja"; }
    function loadLanguageFile(code) { return "{}"; }
    function clearError() { error = ""; }
    function analyzeSpeech() {}
    function predictProsody() {}
    function localFileUrl(path) { return path; }
    function localFilePath(url) { return String(url || "").replace(/^file:\/\/\/([A-Za-z]:)/, "$1").replace(/^file:\/\//, ""); }
}
