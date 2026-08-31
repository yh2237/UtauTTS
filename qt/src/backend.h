#pragma once

#include <QObject>
#include <QByteArray>
#include <QFutureSynchronizer>
#include <QHash>
#include <QList>
#include <QTemporaryDir>
#include <QUrl>
#include <QStringList>
#include <QVariantList>
#include <QVariantMap>
#include <cstdint>

class QFile;
class QNetworkAccessManager;
class QNetworkReply;

class Backend final : public QObject {
    Q_OBJECT
    Q_PROPERTY(bool connected READ connected NOTIFY connectedChanged)
    Q_PROPERTY(bool busy READ busy NOTIFY busyChanged)
    Q_PROPERTY(QString error READ error NOTIFY errorChanged)
    Q_PROPERTY(QVariantList voicebanks READ voicebanks NOTIFY metadataChanged)
    Q_PROPERTY(QVariantList models READ models NOTIFY metadataChanged)
    Q_PROPERTY(QVariantList renderers READ renderers NOTIFY metadataChanged)
    Q_PROPERTY(QVariantList dictionaryEntries READ dictionaryEntries NOTIFY dictionaryChanged)
    Q_PROPERTY(QString defaultRenderer READ defaultRenderer NOTIFY synthesisDefaultsChanged)
    Q_PROPERTY(QString defaultModelId READ defaultModelId NOTIFY synthesisDefaultsChanged)
    Q_PROPERTY(QString defaultVoicebankId READ defaultVoicebankId NOTIFY voicebankSettingsChanged)
    Q_PROPERTY(QString analysisRequestId READ analysisRequestId NOTIFY analysisChanged)
    Q_PROPERTY(QString analysisSourceText READ analysisSourceText NOTIFY analysisChanged)
    Q_PROPERTY(QString analysisJson READ analysisJson NOTIFY analysisChanged)
    Q_PROPERTY(QString prosodyRequestId READ prosodyRequestId NOTIFY prosodyChanged)
    Q_PROPERTY(QString prosodyJson READ prosodyJson NOTIFY prosodyChanged)
    Q_PROPERTY(QString synthesisJson READ synthesisJson NOTIFY synthesisChanged)
    Q_PROPERTY(QUrl previewUrl READ previewUrl NOTIFY previewReady)
    Q_PROPERTY(bool darkMode READ darkMode NOTIFY themeChanged)
    Q_PROPERTY(QString language READ language NOTIFY languageChanged)
    Q_PROPERTY(bool closeLogOnSuccess READ closeLogOnSuccess NOTIFY logSettingsChanged)
    Q_PROPERTY(bool updateCheckEnabled READ updateCheckEnabled NOTIFY updateSettingsChanged)
    Q_PROPERTY(int previewCacheFileCount READ previewCacheFileCount NOTIFY cacheSettingsChanged)
    Q_PROPERTY(bool developerMode READ developerMode NOTIFY developerModeChanged)
    Q_PROPERTY(int defaultMoraDuration READ defaultMoraDuration NOTIFY synthesisDefaultsChanged)
    Q_PROPERTY(int defaultPauseDuration READ defaultPauseDuration NOTIFY synthesisDefaultsChanged)
    Q_PROPERTY(int defaultLeadingPreutterance READ defaultLeadingPreutterance NOTIFY synthesisDefaultsChanged)
    Q_PROPERTY(double defaultIntonationStrength READ defaultIntonationStrength NOTIFY synthesisDefaultsChanged)
    Q_PROPERTY(bool exportTextWithWav READ exportTextWithWav NOTIFY exportSettingsChanged)
    Q_PROPERTY(bool exportLabWithWav READ exportLabWithWav NOTIFY exportSettingsChanged)
    Q_PROPERTY(QString exportTextEncoding READ exportTextEncoding NOTIFY exportSettingsChanged)
    Q_PROPERTY(QString synthesizeShortcut READ synthesizeShortcut NOTIFY shortcutSettingsChanged)
    Q_PROPERTY(QString saveProjectShortcut READ saveProjectShortcut NOTIFY shortcutSettingsChanged)
    Q_PROPERTY(QString reloadVoicebanksShortcut READ reloadVoicebanksShortcut NOTIFY shortcutSettingsChanged)
    Q_PROPERTY(QString addUtteranceShortcut READ addUtteranceShortcut NOTIFY shortcutSettingsChanged)
    Q_PROPERTY(QString removeUtteranceShortcut READ removeUtteranceShortcut NOTIFY shortcutSettingsChanged)
    Q_PROPERTY(QString undoShortcut READ undoShortcut NOTIFY shortcutSettingsChanged)
    Q_PROPERTY(QString redoShortcut READ redoShortcut NOTIFY shortcutSettingsChanged)
    Q_PROPERTY(QStringList logLines READ logLines NOTIFY logsChanged)
public:
    explicit Backend(QObject *parent = nullptr);
    ~Backend() override;
    bool connected() const { return m_handle != 0; }
    bool busy() const { return m_busy; }
    QString error() const { return m_error; }
    QVariantList voicebanks() const { return m_voicebanks; }
    QVariantList models() const { return m_models; }
    QVariantList renderers() const { return m_renderers; }
    QVariantList dictionaryEntries() const { return m_dictionaryEntries; }
    QString defaultRenderer() const { return m_defaultRenderer; }
    QString defaultModelId() const { return m_defaultModelId; }
    QString defaultVoicebankId() const { return m_defaultVoicebankId; }
    QString analysisRequestId() const { return m_analysisRequestId; }
    QString analysisSourceText() const { return m_analysisSourceText; }
    QString analysisJson() const { return m_analysisJson; }
    QString prosodyRequestId() const { return m_prosodyRequestId; }
    QString prosodyJson() const { return m_prosodyJson; }
    QString synthesisJson() const { return m_synthesisJson; }
    QUrl previewUrl() const { return m_previewUrl; }
    bool darkMode() const { return m_darkMode; }
    QString language() const { return m_language; }
    bool closeLogOnSuccess() const { return m_closeLogOnSuccess; }
    bool updateCheckEnabled() const { return m_updateCheckEnabled; }
    int previewCacheFileCount() const { return m_previewCacheFileCount; }
    bool developerMode() const { return m_developerMode; }
    int defaultMoraDuration() const { return m_defaultMoraDuration; }
    int defaultPauseDuration() const { return m_defaultPauseDuration; }
    int defaultLeadingPreutterance() const { return m_defaultLeadingPreutterance; }
    double defaultIntonationStrength() const { return m_defaultIntonationStrength; }
    bool exportTextWithWav() const { return m_exportTextWithWav; }
    bool exportLabWithWav() const { return m_exportLabWithWav; }
    QString exportTextEncoding() const { return m_exportTextEncoding; }
    QString synthesizeShortcut() const { return m_synthesizeShortcut; }
    QString saveProjectShortcut() const { return m_saveProjectShortcut; }
    QString reloadVoicebanksShortcut() const { return m_reloadVoicebanksShortcut; }
    QString addUtteranceShortcut() const { return m_addUtteranceShortcut; }
    QString removeUtteranceShortcut() const { return m_removeUtteranceShortcut; }
    QString undoShortcut() const { return m_undoShortcut; }
    QString redoShortcut() const { return m_redoShortcut; }
    QStringList logLines() const { return m_logLines; }

    Q_INVOKABLE void initialize();
    Q_INVOKABLE void reloadVoicebanks();
    Q_INVOKABLE QString addExternalRenderer(const QUrl &executable);
    Q_INVOKABLE bool removeExternalRenderer(const QString &id);
    Q_INVOKABLE bool openVoiceDirectory();
    Q_INVOKABLE void analyze(const QString &text, const QString &requestId);
    Q_INVOKABLE void predictProsody(const QVariantMap &request);
    Q_INVOKABLE void synthesize(const QVariantMap &request);
    Q_INVOKABLE bool savePreview(const QUrl &destination);
    Q_INVOKABLE bool startFileDrag(const QVariantList &files);
    Q_INVOKABLE QUrl writeDragExo(const QUrl &directory, const QVariantList &files, int frameRate);
    Q_INVOKABLE QUrl defaultSaveFile(const QString &fileName) const;
    Q_INVOKABLE QUrl fileInDirectory(const QUrl &directory, const QString &fileName) const;
    Q_INVOKABLE bool saveProject(const QUrl &destination, const QVariantMap &project);
    Q_INVOKABLE void exportUstx(const QUrl &destination, const QVariantMap &project);
    Q_INVOKABLE QVariantMap loadProject(const QUrl &source);
    Q_INVOKABLE bool exportDiagnosticReport(const QUrl &destination, const QVariantMap &context);
    Q_INVOKABLE QVariantMap loadProsodyPromptSet() const;
    Q_INVOKABLE QVariantMap loadProsodyTrainingSession();
    Q_INVOKABLE bool saveProsodyTrainingSession(const QVariantMap &session);
    Q_INVOKABLE bool clearProsodyTrainingSession();
    Q_INVOKABLE bool exportProsodyTrainingDataset(const QUrl &destination, const QVariantMap &session);
    Q_INVOKABLE QString dictionaryFingerprint() const;
    Q_INVOKABLE void setDarkMode(bool value);
    Q_INVOKABLE void setLanguage(const QString &value);
    Q_INVOKABLE QString resolvedLanguage() const;
    Q_INVOKABLE QString loadLanguageFile(const QString &code) const;
    Q_INVOKABLE QStringList languageCodes() const;
    Q_INVOKABLE QString languageDisplayName(const QString &code) const;
    Q_INVOKABLE QString suppressedUpdateVersion() const;
    Q_INVOKABLE void setSuppressedUpdateVersion(const QString &version);
    Q_INVOKABLE bool showNativeAboutDialog();
    Q_INVOKABLE bool startUpdateDownload(const QString &downloadUrl, const QString &version);
    Q_INVOKABLE bool installUpdate(const QString &localZip, const QString &version);
    Q_INVOKABLE void cancelUpdateDownload();
    void showUpdateError(const QString &title, const QString &text);
    Q_INVOKABLE void clearLogs();
    Q_INVOKABLE void setCloseLogOnSuccess(bool value);
    Q_INVOKABLE void setUpdateCheckEnabled(bool value);
    Q_INVOKABLE void setPreviewCacheFileCount(int value);
    Q_INVOKABLE void setDeveloperMode(bool value);
    Q_INVOKABLE void setSynthesisDefaults(int moraDuration, int pauseDuration,
                                          int leadingPreutterance, double intonationStrength,
                                          const QString &modelId, const QString &rendererId);
    Q_INVOKABLE void setDefaultVoicebank(const QString &value);
    Q_INVOKABLE void setExportSettings(bool writeText, bool writeLab, const QString &textEncoding);
    Q_INVOKABLE void setShortcutSequences(const QString &synthesize,
                                          const QString &saveProject,
                                          const QString &reloadVoicebanks,
                                          const QString &addUtterance,
                                          const QString &removeUtterance,
                                          const QString &undo,
                                          const QString &redo);
    Q_INVOKABLE void setDictionaryEntries(const QVariantList &entries);

signals:
    void connectedChanged();
    void busyChanged();
    void errorChanged();
    void metadataChanged();
    void analysisChanged();
    void prosodyChanged();
    void synthesisChanged();
    void previewReady();
    void themeChanged();
    void languageChanged();
    void logSettingsChanged();
    void updateSettingsChanged();
    void cacheSettingsChanged();
    void developerModeChanged();
    void synthesisDefaultsChanged();
    void exportSettingsChanged();
    void voicebankSettingsChanged();
    void ustxExportFinished(bool success, const QString &detail);
    void shortcutSettingsChanged();
    void dictionaryChanged();
    void logsChanged();
    void updateDownloadProgress(qint64 bytesReceived, qint64 bytesTotal);
    void updateDownloadFinished(bool success, const QString &localZip);

private:
    struct PreviewCacheEntry {
        QString path;
        QString text;
        QString lab;
        QString synthesisJson;
    };

    QVariantMap call(const QByteArray &method, const QVariantMap &request = {});
    void refreshMetadata();
    void setBusy(bool value);
    void setError(const QString &value);
    QByteArray previewCacheKey(const QVariantMap &request) const;
    bool restorePreviewCache(const QByteArray &key);
    void storePreviewCache(const QByteArray &key, const PreviewCacheEntry &entry);
    void trimPreviewCache();
    void clearPreviewCache();
    bool restartNativeBackend();
    uintptr_t m_handle = 0;
    bool m_busy = false;
    QString m_error;
    QString m_previewPath;
    QString m_previewText;
    QString m_previewLab;
    QTemporaryDir m_previewDirectory;
    QHash<QByteArray, PreviewCacheEntry> m_previewCache;
    QList<QByteArray> m_previewCacheOrder;
    QFutureSynchronizer<QVariantMap> m_activeCalls;
    int m_activeCallCount = 0;
    QVariantList m_voicebanks, m_models, m_renderers, m_dictionaryEntries;
    QString m_catalogDefaultRenderer;
    QString m_defaultRenderer;
    QString m_defaultModelId;
    QString m_defaultVoicebankId;
    QString m_analysisRequestId, m_analysisSourceText, m_analysisJson;
    QString m_prosodyRequestId, m_prosodyJson;
    QString m_synthesisJson;
    QUrl m_previewUrl;
    bool m_darkMode = false;
    QString m_language;
    mutable QHash<QString, QString> m_languageNames;
    mutable bool m_languageNamesLoaded = false;
    bool m_closeLogOnSuccess = true;
    bool m_updateCheckEnabled = true;
    int m_previewCacheFileCount = 32;
    bool m_developerMode = false;
    int m_defaultMoraDuration = 120;
    int m_defaultPauseDuration = 180;
    int m_defaultLeadingPreutterance = 0;
    double m_defaultIntonationStrength = 2.0;
    bool m_exportTextWithWav = false;
    bool m_exportLabWithWav = false;
    QString m_exportTextEncoding = QStringLiteral("utf-8");
    QString m_synthesizeShortcut;
    QString m_saveProjectShortcut;
    QString m_reloadVoicebanksShortcut;
    QString m_addUtteranceShortcut;
    QString m_removeUtteranceShortcut;
    QString m_undoShortcut;
    QString m_redoShortcut;
    QStringList m_logLines;
    QNetworkAccessManager *m_updateNetwork = nullptr;
    QNetworkReply *m_updateReply = nullptr;
    QFile *m_updateFile = nullptr;
    bool m_updateCancelled = false;
    QString m_updateWriteError;
    QHash<QString, quint64> m_analysisGenerations;
    quint64 m_nextAnalysisGeneration = 0;
    quint64 m_nextProsodyGeneration = 0;

    void appendLog(const QString &message);
    QHash<QString, QString> languageDisplayNames() const;
};
