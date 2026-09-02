#include "backend.h"
#include "utautts_abi.h"

#include <QCoreApplication>
#include <QCryptographicHash>
#include <QDateTime>
#include <QDesktopServices>
#include <QDir>
#include <QDrag>
#include <QFile>
#include <QFileInfo>
#include <QMimeData>
#include <QFutureWatcher>
#include <QJsonDocument>
#include <QJsonArray>
#include <QJsonObject>
#include <QLocale>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QProcess>
#include <QRegularExpression>
#include <QSaveFile>
#include <QStandardPaths>
#include <QSettings>
#include <QSysInfo>
#include <QUuid>
#include <QtConcurrent>
#include <algorithm>
#include <memory>
#include <stdexcept>

#ifdef Q_OS_WIN
#include <windows.h>
#endif

namespace {
constexpr int maxRecentProjects = 10;

QDir resourceRoot();

QString sanitizeLanguageCode(const QString &code) {
    const QString lower = code.trimmed().toLower();
    return lower.isEmpty() ? QStringLiteral("auto") : lower;
}

QString normalizeAliasPolicySetting(const QString &value) {
    const QString normalized = value.trimmed().toLower();
    const QStringList supported{
        QStringLiteral("auto"), QStringLiteral("legacy"), QStringLiteral("cvvc-enhanced"),
        QStringLiteral("vcv-prefer"), QStringLiteral("cvvc-prefer"), QStringLiteral("cv-only"),
    };
    return supported.contains(normalized) ? normalized : QStringLiteral("auto");
}

QStringList availableLanguageCodes() {
    const QStringList files = QDir(QStringLiteral(":/lang"))
            .entryList({QStringLiteral("*.json")}, QDir::Files, QDir::Name);
    QStringList codes;
    for (const QString &file : files) {
        if (file.compare(QLatin1String("lang.json"), Qt::CaseInsensitive) == 0)
            continue;
        codes.append(file.left(file.size() - QStringLiteral(".json").size()).toLower());
    }
    return codes;
}

QString languageCodeForLocale(const QString &locale, const QStringList &available) {
    QString normalized = locale.trimmed().toLower();
    normalized.replace(QLatin1Char('-'), QLatin1Char('_'));
    if (available.contains(normalized))
        return normalized;
    const qsizetype separator = normalized.indexOf(QLatin1Char('_'));
    if (separator > 0) {
        const QString language = normalized.left(separator);
        if (available.contains(language))
            return language;
    }
    return QString();
}

bool hasResourceLayout(const QDir &root) {
    return root.exists("plugins/renderers") || root.exists("models") || root.exists("voice");
}

QString prosodyTrainingSessionPath() {
    QString directory = qEnvironmentVariable("UTAUTTS_SELF_TEST_DIRECTORY");
    if (directory.isEmpty())
        directory = resourceRoot().absolutePath();
    return QDir(directory).filePath(QStringLiteral("prosody-training-session.json"));
}

QString externalRendererRootPath() {
    QString directory = qEnvironmentVariable("UTAUTTS_SELF_TEST_DIRECTORY");
    if (directory.isEmpty())
        return resourceRoot().filePath(QStringLiteral("plugins/renderers"));
    return QDir(directory).filePath(QStringLiteral("renderers"));
}

bool writeJSONFile(const QString &path, const QVariantMap &value, QString *error) {
    const QJsonDocument document = QJsonDocument::fromVariant(value);
    if (!document.isObject()) {
        if (error)
            *error = QStringLiteral("invalid JSON object");
        return false;
    }
    QDir().mkpath(QFileInfo(path).absolutePath());
    const QByteArray data = document.toJson(QJsonDocument::Indented);
    QSaveFile target(path);
    if (!target.open(QIODevice::WriteOnly) || target.write(data) != data.size() || !target.commit()) {
        if (error)
            *error = target.errorString();
        target.cancelWriting();
        return false;
    }
    return true;
}

QString updateLockPath(const QDir &root) {
    return root.absolutePath() + QStringLiteral(".update-lock.json");
}

bool writePendingUpdateLock(const QDir &root, const QString &version, QString *error) {
    QSaveFile file(updateLockPath(root));
    if (!file.open(QIODevice::WriteOnly)) {
        if (error)
            *error = file.errorString();
        return false;
    }
    const QJsonObject state{
        {QStringLiteral("version"), version},
        {QStringLiteral("started_at"), QDateTime::currentDateTimeUtc().toString(Qt::ISODateWithMs)},
        {QStringLiteral("updater_pid"), 0},
    };
    const QByteArray data = QJsonDocument(state).toJson(QJsonDocument::Compact) + '\n';
    if (file.write(data) != data.size() || !file.commit()) {
        if (error)
            *error = file.errorString();
        file.cancelWriting();
        return false;
    }
    return true;
}

QDir resourceRoot() {
    QDir application(QCoreApplication::applicationDirPath());
    if (application.dirName().compare("app", Qt::CaseInsensitive) == 0) {
        application.cdUp();
    }
    if (hasResourceLayout(application)) {
        return application;
    }

    QDir candidate(QDir::current());
    for (int depth = 0; depth < 8; ++depth) {
        if (hasResourceLayout(candidate)) {
            return candidate;
        }
        if (!candidate.cdUp()) {
            break;
        }
    }
    return application;
}

QString portableSettingsPath() {
    const QString selfTestDirectory = qEnvironmentVariable("UTAUTTS_SELF_TEST_DIRECTORY");
    if (!selfTestDirectory.isEmpty())
        return QDir(selfTestDirectory).filePath(QStringLiteral("config.ini"));
    return resourceRoot().filePath(QStringLiteral("config.ini"));
}

void ensurePortableSettings() {
    static bool initialized = false;
    if (initialized)
        return;
    initialized = true;
    const QString path = portableSettingsPath();
    if (QFileInfo::exists(path))
        return;

    QSettings portable(path, QSettings::IniFormat);
    portable.setValue(QStringLiteral("format/version"), 1);
    portable.sync();
}

QVariant portableSettingValue(const QString &key, const QVariant &defaultValue = {}) {
    ensurePortableSettings();
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    return settings.value(key, defaultValue);
}
}

Backend::Backend(QObject *parent)
    : QObject(parent),
      m_darkMode(portableSettingValue("appearance/darkMode", false).toBool()),
      m_language(portableSettingValue("appearance/language", QStringLiteral("auto")).toString()),
      m_closeLogOnSuccess(portableSettingValue("logging/closeOnSuccess", true).toBool()),
      m_updateCheckEnabled(portableSettingValue("appearance/updateCheckEnabled", true).toBool()),
      m_previewCacheFileCount(portableSettingValue("performance/previewCacheFileCount", 32).toInt()),
      m_developerMode(portableSettingValue("developer/enabled", false).toBool()),
      m_defaultRenderer(portableSettingValue("synthesis/defaultRendererId",
                                          QStringLiteral("utautts-world-phrase")).toString().trimmed()),
      m_defaultModelId(portableSettingValue("synthesis/defaultModelId",
                                         QStringLiteral("frame-intonation-v8")).toString().trimmed()),
      m_defaultVoicebankId(portableSettingValue("voicebank/defaultId", QString()).toString().trimmed()),
      m_defaultMoraDuration(portableSettingValue("synthesis/defaultMoraDuration", 120).toInt()),
      m_defaultPauseDuration(portableSettingValue("synthesis/defaultPauseDuration", 180).toInt()),
      m_defaultLeadingPreutterance(portableSettingValue("synthesis/defaultLeadingPreutterance", 0).toInt()),
      m_defaultIntonationStrength(portableSettingValue("synthesis/defaultIntonationStrength", 2.0).toDouble()),
      m_defaultTone(portableSettingValue("synthesis/defaultTone", QStringLiteral("C4")).toString().trimmed()),
      m_defaultAliasPolicy(normalizeAliasPolicySetting(
          portableSettingValue("synthesis/defaultAliasPolicy", QStringLiteral("auto")).toString())),
      m_exportTextWithWav(portableSettingValue("export/writeTextWithWav", false).toBool()),
      m_exportLabWithWav(portableSettingValue("export/writeLabWithWav", false).toBool()),
      m_exportTextEncoding(portableSettingValue("export/textEncoding", QStringLiteral("utf-8")).toString().trimmed().toLower()),
      m_synthesizeShortcut(portableSettingValue("shortcuts/synthesize", QStringLiteral("Ctrl+Enter")).toString()),
      m_saveProjectShortcut(portableSettingValue("shortcuts/saveProject", QStringLiteral("Ctrl+S")).toString()),
      m_reloadVoicebanksShortcut(portableSettingValue("shortcuts/reloadVoicebanks", QStringLiteral("Ctrl+O")).toString()),
      m_addUtteranceShortcut(portableSettingValue("shortcuts/addUtterance", QStringLiteral("Ctrl+D")).toString()),
      m_removeUtteranceShortcut(portableSettingValue("shortcuts/removeUtterance", QStringLiteral("Delete")).toString()),
      m_undoShortcut(portableSettingValue("shortcuts/undo", QStringLiteral("Ctrl+Z")).toString()),
      m_redoShortcut(portableSettingValue("shortcuts/redo", QStringLiteral("Ctrl+Y")).toString()),
      m_recentProjects(portableSettingValue("projects/recent", QStringList()).toStringList()),
      m_updateNetwork(new QNetworkAccessManager(this)) {
    m_defaultMoraDuration = qBound(20, m_defaultMoraDuration, 1000);
    m_defaultPauseDuration = qBound(0, m_defaultPauseDuration, 3000);
    m_defaultLeadingPreutterance = qBound(0, m_defaultLeadingPreutterance, 300);
    m_defaultIntonationStrength = qBound(0.0, m_defaultIntonationStrength, 4.0);
    if (m_defaultTone.isEmpty())
        m_defaultTone = QStringLiteral("C4");
    m_previewCacheFileCount = qBound(1, m_previewCacheFileCount, 256);
    if (m_exportTextEncoding != QStringLiteral("shift_jis"))
        m_exportTextEncoding = QStringLiteral("utf-8");
    QStringList existingProjects;
    for (const QString &path : m_recentProjects) {
        const QString absolutePath = QFileInfo(path).absoluteFilePath();
        if (QFileInfo(absolutePath).isFile() && !existingProjects.contains(absolutePath))
            existingProjects.append(absolutePath);
        if (existingProjects.size() >= maxRecentProjects)
            break;
    }
    if (m_recentProjects != existingProjects) {
        m_recentProjects = existingProjects;
        QSettings settings(portableSettingsPath(), QSettings::IniFormat);
        settings.setValue(QStringLiteral("projects/recent"), m_recentProjects);
        settings.sync();
    }
    const QByteArray dictionaryJSON = portableSettingValue("dictionary/entries").toByteArray();
    QJsonParseError parseError;
    const QJsonDocument dictionaryDocument = QJsonDocument::fromJson(dictionaryJSON, &parseError);
    if (parseError.error == QJsonParseError::NoError && dictionaryDocument.isArray()) {
        for (const QJsonValue &value : dictionaryDocument.array()) {
            const QVariantMap entry = value.toObject().toVariantMap();
            const QString surface = entry.value("surface").toString().trimmed();
            const QString reading = entry.value("reading").toString().trimmed();
            if (!surface.isEmpty() && !reading.isEmpty()) {
                m_dictionaryEntries.append(QVariantMap{{"surface", surface}, {"reading", reading}});
            }
        }
    }
}
Backend::~Backend() {
    if (m_updateReply) {
        m_updateReply->abort();
    }
    m_activeCalls.waitForFinished();
    if (m_handle) {
        UtauTTSDestroy(m_handle);
    }
}

void Backend::setDarkMode(bool value) {
    if (m_darkMode == value) {
        return;
    }
    m_darkMode = value;
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.setValue("appearance/darkMode", value);
    settings.sync();
    emit themeChanged();
}

void Backend::setLanguage(const QString &value) {
    const QString normalized = sanitizeLanguageCode(value);
    if (m_language == normalized) {
        return;
    }
    m_language = normalized;
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.setValue("appearance/language", normalized);
    settings.sync();
    emit languageChanged();
}

QString Backend::loadLanguageFile(const QString &code) const {
    const QString selected = sanitizeLanguageCode(code);
    const QString cleaned = selected == QStringLiteral("auto") ? resolvedLanguage() : selected;
    QFile file(QStringLiteral(":/lang/%1.json").arg(cleaned));
    if (!file.open(QIODevice::ReadOnly | QIODevice::Text)) {
        QFile fallback(QStringLiteral(":/lang/en.json"));
        if (!fallback.open(QIODevice::ReadOnly | QIODevice::Text))
            return QString();
        return QString::fromUtf8(fallback.readAll());
    }
    return QString::fromUtf8(file.readAll());
}

QStringList Backend::languageCodes() const {
    QStringList codes = availableLanguageCodes();
    if (codes.isEmpty()) {
        codes = languageDisplayNames().keys();
        codes.sort();
    }
    codes.removeAll(QStringLiteral("auto"));
    codes.prepend(QStringLiteral("auto"));
    return codes;
}

QString Backend::resolvedLanguage() const {
    const QString selected = sanitizeLanguageCode(m_language);
    const QStringList available = availableLanguageCodes();
    if (selected != QStringLiteral("auto"))
        return available.contains(selected) ? selected : QStringLiteral("en");

    QStringList locales = QLocale::system().uiLanguages();
    locales.append(QLocale::system().name());
    for (const QString &locale : locales) {
        const QString matched = languageCodeForLocale(locale, available);
        if (!matched.isEmpty())
            return matched;
    }
    return QStringLiteral("en");
}

QHash<QString, QString> Backend::languageDisplayNames() const {
    if (!m_languageNamesLoaded) {
        m_languageNamesLoaded = true;
        QFile file(QStringLiteral(":/lang/lang.json"));
        if (file.open(QIODevice::ReadOnly | QIODevice::Text)) {
            const QJsonDocument document = QJsonDocument::fromJson(file.readAll());
            if (document.isObject()) {
                const QJsonObject object = document.object();
                for (auto it = object.begin(); it != object.end(); ++it)
                    m_languageNames.insert(it.key(), it.value().toString());
            }
        }
    }
    return m_languageNames;
}

QString Backend::languageDisplayName(const QString &code) const {
    const QString cleaned = sanitizeLanguageCode(code);
    if (cleaned == QStringLiteral("auto"))
        return QStringLiteral("Automatic");
    return languageDisplayNames().value(cleaned, cleaned);
}

QString Backend::suppressedUpdateVersion() const {
    return portableSettingValue(QStringLiteral("appearance/suppressedUpdateVersion"), QString()).toString();
}

void Backend::setSuppressedUpdateVersion(const QString &version) {
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.setValue(QStringLiteral("appearance/suppressedUpdateVersion"), version);
    settings.sync();
}

void Backend::setCloseLogOnSuccess(bool value) {
    if (m_closeLogOnSuccess == value) {
        return;
    }
    m_closeLogOnSuccess = value;
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.setValue("logging/closeOnSuccess", value);
    settings.sync();
    emit logSettingsChanged();
}

void Backend::setUpdateCheckEnabled(bool value) {
    if (m_updateCheckEnabled == value) {
        return;
    }
    m_updateCheckEnabled = value;
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.setValue("appearance/updateCheckEnabled", value);
    settings.sync();
    emit updateSettingsChanged();
}

void Backend::setDeveloperMode(bool value) {
    if (m_developerMode == value)
        return;
    m_developerMode = value;
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.setValue(QStringLiteral("developer/enabled"), value);
    settings.sync();
    emit developerModeChanged();
}

void Backend::setDefaultVoicebank(const QString &value) {
    const QString normalized = value.trimmed();
    if (m_defaultVoicebankId == normalized) {
        return;
    }
    m_defaultVoicebankId = normalized;
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.setValue("voicebank/defaultId", m_defaultVoicebankId);
    settings.sync();
    emit voicebankSettingsChanged();
}

void Backend::setPreviewCacheFileCount(int value) {
    const int bounded = qBound(1, value, 256);
    if (m_previewCacheFileCount == bounded)
        return;
    m_previewCacheFileCount = bounded;
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.setValue(QStringLiteral("performance/previewCacheFileCount"), bounded);
    settings.sync();
    trimPreviewCache();
    emit cacheSettingsChanged();
}

void Backend::setExportSettings(bool writeText, bool writeLab, const QString &textEncoding) {
    const QString normalizedEncoding = textEncoding.trimmed().toLower() == QStringLiteral("shift_jis")
            ? QStringLiteral("shift_jis") : QStringLiteral("utf-8");
    if (m_exportTextWithWav == writeText
            && m_exportLabWithWav == writeLab
            && m_exportTextEncoding == normalizedEncoding) {
        return;
    }
    m_exportTextWithWav = writeText;
    m_exportLabWithWav = writeLab;
    m_exportTextEncoding = normalizedEncoding;
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.setValue("export/writeTextWithWav", m_exportTextWithWav);
    settings.setValue("export/writeLabWithWav", m_exportLabWithWav);
    settings.setValue("export/textEncoding", m_exportTextEncoding);
    settings.sync();
    emit exportSettingsChanged();
}

void Backend::setSynthesisDefaults(int moraDuration, int pauseDuration,
                                   int leadingPreutterance, double intonationStrength,
                                   const QString &modelId, const QString &rendererId,
                                   const QString &tone, const QString &aliasPolicy) {
    const int boundedMoraDuration = qBound(20, moraDuration, 1000);
    const int boundedPauseDuration = qBound(0, pauseDuration, 3000);
    const int boundedLeadingPreutterance = qBound(0, leadingPreutterance, 300);
    const double boundedIntonationStrength = qBound(0.0, intonationStrength, 4.0);
    const QString normalizedModelId = modelId.trimmed();
    const QString normalizedRendererId = rendererId.trimmed();
    const QString normalizedTone = tone.trimmed().isEmpty() ? QStringLiteral("C4") : tone.trimmed();
    const QString normalizedAliasPolicy = normalizeAliasPolicySetting(aliasPolicy);
    if (m_defaultMoraDuration == boundedMoraDuration
            && m_defaultPauseDuration == boundedPauseDuration
            && m_defaultLeadingPreutterance == boundedLeadingPreutterance
            && qFuzzyCompare(m_defaultIntonationStrength, boundedIntonationStrength)
            && m_defaultModelId == normalizedModelId
            && m_defaultRenderer == normalizedRendererId
            && m_defaultTone == normalizedTone
            && m_defaultAliasPolicy == normalizedAliasPolicy) {
        return;
    }
    m_defaultMoraDuration = boundedMoraDuration;
    m_defaultPauseDuration = boundedPauseDuration;
    m_defaultLeadingPreutterance = boundedLeadingPreutterance;
    m_defaultIntonationStrength = boundedIntonationStrength;
    m_defaultModelId = normalizedModelId;
    m_defaultRenderer = normalizedRendererId;
    m_defaultTone = normalizedTone;
    m_defaultAliasPolicy = normalizedAliasPolicy;
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.setValue("synthesis/defaultMoraDuration", m_defaultMoraDuration);
    settings.setValue("synthesis/defaultPauseDuration", m_defaultPauseDuration);
    settings.setValue("synthesis/defaultLeadingPreutterance", m_defaultLeadingPreutterance);
    settings.setValue("synthesis/defaultIntonationStrength", m_defaultIntonationStrength);
    settings.remove("synthesis/defaultApplyPitch");
    settings.setValue("synthesis/defaultModelId", m_defaultModelId);
    settings.setValue("synthesis/defaultRendererId", m_defaultRenderer);
    settings.setValue("synthesis/defaultTone", m_defaultTone);
    settings.setValue("synthesis/defaultAliasPolicy", m_defaultAliasPolicy);
    settings.sync();
    emit synthesisDefaultsChanged();
}

void Backend::setShortcutSequences(const QString &synthesize,
                                   const QString &saveProject,
                                   const QString &reloadVoicebanks,
                                   const QString &addUtterance,
                                   const QString &removeUtterance,
                                   const QString &undo,
                                   const QString &redo) {
    if (m_synthesizeShortcut == synthesize.trimmed()
            && m_saveProjectShortcut == saveProject.trimmed()
            && m_reloadVoicebanksShortcut == reloadVoicebanks.trimmed()
            && m_addUtteranceShortcut == addUtterance.trimmed()
            && m_removeUtteranceShortcut == removeUtterance.trimmed()
            && m_undoShortcut == undo.trimmed()
            && m_redoShortcut == redo.trimmed()) {
        return;
    }
    m_synthesizeShortcut = synthesize.trimmed();
    m_saveProjectShortcut = saveProject.trimmed();
    m_reloadVoicebanksShortcut = reloadVoicebanks.trimmed();
    m_addUtteranceShortcut = addUtterance.trimmed();
    m_removeUtteranceShortcut = removeUtterance.trimmed();
    m_undoShortcut = undo.trimmed();
    m_redoShortcut = redo.trimmed();
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.setValue("shortcuts/synthesize", m_synthesizeShortcut);
    settings.setValue("shortcuts/saveProject", m_saveProjectShortcut);
    settings.setValue("shortcuts/reloadVoicebanks", m_reloadVoicebanksShortcut);
    settings.setValue("shortcuts/addUtterance", m_addUtteranceShortcut);
    settings.setValue("shortcuts/removeUtterance", m_removeUtteranceShortcut);
    settings.setValue("shortcuts/undo", m_undoShortcut);
    settings.setValue("shortcuts/redo", m_redoShortcut);
    settings.sync();
    emit shortcutSettingsChanged();
}

void Backend::setDictionaryEntries(const QVariantList &entries) {
    QVariantList normalized;
    for (const QVariant &value : entries) {
        const QVariantMap entry = value.toMap();
        const QString surface = entry.value("surface").toString().trimmed();
        const QString reading = entry.value("reading").toString().trimmed();
        if (!surface.isEmpty() && !reading.isEmpty()) {
            normalized.append(QVariantMap{{"surface", surface}, {"reading", reading}});
        }
    }
    if (m_dictionaryEntries == normalized) {
        return;
    }
    m_dictionaryEntries = normalized;
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.setValue("dictionary/entries", QJsonDocument(QJsonArray::fromVariantList(m_dictionaryEntries)).toJson(QJsonDocument::Compact));
    settings.sync();
    emit dictionaryChanged();
}

void Backend::appendLog(const QString &message) {
    if (message.trimmed().isEmpty()) {
        return;
    }
    const QString timestamp = QDateTime::currentDateTime().toString("HH:mm:ss");
    m_logLines.append(QStringLiteral("[%1] %2").arg(timestamp, message));
    constexpr int maxLogLines = 500;
    while (m_logLines.size() > maxLogLines) {
        m_logLines.removeFirst();
    }
    emit logsChanged();
}

void Backend::clearLogs() {
    if (m_logLines.isEmpty()) {
        return;
    }
    m_logLines.clear();
    emit logsChanged();
}

bool Backend::showNativeAboutDialog() {
#ifdef Q_OS_WIN
    const QString title = tr("UtauTTSについて");
    const QString text = QStringLiteral("UtauTTS %1 \n\nDeveloped by yh（@2237yh）\nTesting by アアアアアアア（@a7_riri）\n\nUTAUボイスバンクの原音接続に、学習ベースのイントネーション調整を加えた日本語TTS").arg(QCoreApplication::applicationVersion());
    MessageBoxW(GetActiveWindow(),
                reinterpret_cast<LPCWSTR>(text.utf16()),
                reinterpret_cast<LPCWSTR>(title.utf16()),
                MB_OK | MB_ICONINFORMATION);
    return true;
#else
    return false;
#endif
}

bool Backend::startUpdateDownload(const QString &downloadUrl, const QString &version) {
    if (downloadUrl.isEmpty()) {
        const QString message = tr("ダウンロードURLを取得できませんでした。");
        setError(message);
        showUpdateError(tr("更新に失敗しました"), message);
        return false;
    }
    QString safeVersion = version.trimmed();
    safeVersion.replace(QRegularExpression(QStringLiteral("[^A-Za-z0-9._-]")), QStringLiteral("_"));
    const QString fileName = safeVersion.isEmpty()
        ? QStringLiteral("utautts-update.zip")
        : QStringLiteral("utautts-update-%1.zip").arg(safeVersion);
    const QString zipPath = QDir(QDir::tempPath()).filePath(fileName);

    if (m_updateReply) {
        m_updateReply->abort();
        m_updateReply->deleteLater();
        m_updateReply = nullptr;
    }
    delete m_updateFile;
    m_updateFile = nullptr;
    m_updateCancelled = false;
    m_updateWriteError.clear();
    QFile::remove(zipPath);
    m_updateFile = new QFile(zipPath, this);
    if (!m_updateFile->open(QIODevice::WriteOnly | QIODevice::Truncate)) {
        delete m_updateFile;
        m_updateFile = nullptr;
        const QString message = tr("ダウンロード先の一時ファイルを開けませんでした。");
        setError(message);
        showUpdateError(tr("更新に失敗しました"), message);
        return false;
    }

    QNetworkRequest request{QUrl(downloadUrl)};
    request.setTransferTimeout(10 * 60 * 1000);
    QNetworkReply *reply = m_updateNetwork->get(request);
    m_updateReply = reply;
    connect(reply, &QNetworkReply::readyRead, this, [this, reply] {
        if (m_updateReply == reply && m_updateFile) {
            const QByteArray data = reply->readAll();
            if (!data.isEmpty() && m_updateFile->write(data) != data.size()) {
                m_updateWriteError = m_updateFile->errorString();
                if (m_updateWriteError.isEmpty())
                    m_updateWriteError = tr("更新ファイルを一時ディレクトリへ書き込めませんでした。");
                reply->abort();
            }
        }
    });
    connect(reply, &QNetworkReply::downloadProgress, this, [this, reply](qint64 received, qint64 total) {
        if (m_updateReply == reply) {
            emit updateDownloadProgress(received, total);
        }
    });
    connect(reply, &QNetworkReply::finished, this, [this, reply] {
        if (m_updateReply != reply || !m_updateFile) {
            return;
        }
        const QNetworkReply::NetworkError networkError = reply->error();
        const bool cancelled = m_updateCancelled && networkError == QNetworkReply::OperationCanceledError;
        const QString errorText = reply->errorString();
        const QString path = m_updateFile->fileName();
        if (!m_updateFile->flush() && m_updateWriteError.isEmpty())
            m_updateWriteError = m_updateFile->errorString();
        const QString writeError = m_updateWriteError;
        delete m_updateFile;
        m_updateFile = nullptr;
        m_updateReply = nullptr;
        m_updateCancelled = false;
        m_updateWriteError.clear();
        reply->deleteLater();
        if (cancelled) {
            QFile::remove(path);
            return;
        }
        if (networkError == QNetworkReply::NoError && writeError.isEmpty()) {
            emit updateDownloadFinished(true, path);
            return;
        }
        QFile::remove(path);
        const QString detail = !writeError.isEmpty()
            ? writeError
            : networkError == QNetworkReply::OperationCanceledError
            ? tr("タイムアウトしました。通信環境を確認して、再度お試しください。")
            : errorText;
        setError(tr("更新ファイルのダウンロードに失敗しました: %1").arg(detail));
        showUpdateError(tr("更新に失敗しました"), tr("更新ファイルのダウンロードに失敗しました。\n%1\n\nリリースページから手動で更新するか、時間をおいて再度お試しください。").arg(detail));
        emit updateDownloadFinished(false, QString());
    });
    return true;
}

bool Backend::installUpdate(const QString &localZip, const QString &version) {
    const QDir root = resourceRoot();
#ifdef Q_OS_WIN
    const QString updaterName = QStringLiteral("utautts-updater.exe");
#else
    const QString updaterName = QStringLiteral("utautts-updater");
#endif
    const QString updaterPath = root.filePath(QStringLiteral("tools/") + updaterName);
    if (!QFileInfo::exists(updaterPath)) {
        const QString message = tr("アップデータが同梱されていません。リリースページから手動で更新してください。");
        setError(message);
        showUpdateError(tr("更新に失敗しました"), message);
        return false;
    }
    if (localZip.isEmpty() || !QFileInfo::exists(localZip)) {
        const QString message = tr("ダウンロードした更新ファイルが見つかりません。");
        setError(message);
        showUpdateError(tr("更新に失敗しました"), message);
        return false;
    }
    const QString tempUpdater = QDir(QDir::tempPath()).filePath(
        QStringLiteral("utautts-updater-%1%2").arg(QCoreApplication::applicationPid())
#ifdef Q_OS_WIN
        .arg(QStringLiteral(".exe")));
#else
        .arg(QString()));
#endif
    QFile::remove(tempUpdater);
    if (!QFile::copy(updaterPath, tempUpdater)) {
        const QString message = tr("アップデータを一時ディレクトリに配置できませんでした。");
        setError(message);
        showUpdateError(tr("更新に失敗しました"), message);
        return false;
    }
#ifndef Q_OS_WIN
    if (!QFile::setPermissions(tempUpdater, QFileDevice::ReadOwner | QFileDevice::WriteOwner
                              | QFileDevice::ExeOwner | QFileDevice::ReadGroup | QFileDevice::ExeGroup
                              | QFileDevice::ReadOther | QFileDevice::ExeOther)) {
        QFile::remove(tempUpdater);
        const QString message = tr("アップデーターへ実行権限を設定できませんでした。");
        setError(message);
        showUpdateError(tr("更新に失敗しました"), message);
        return false;
    }
#endif
    const QStringList arguments{
        QStringLiteral("-target"), QDir::toNativeSeparators(root.absolutePath()),
        QStringLiteral("-zip"), QDir::toNativeSeparators(localZip),
        QStringLiteral("-delete-zip"),
        QStringLiteral("-pid"), QString::number(QCoreApplication::applicationPid()),
        QStringLiteral("-version"), version,
    };
    QString lockError;
    if (!writePendingUpdateLock(root, version, &lockError)) {
        const QString message = tr("更新中ロックを作成できませんでした: %1").arg(lockError);
        setError(message);
        showUpdateError(tr("更新に失敗しました"), message);
        return false;
    }
    qint64 updaterPid = 0;
    if (!QProcess::startDetached(tempUpdater, arguments, QDir::tempPath(), &updaterPid)) {
        QFile::remove(updateLockPath(root));
        const QString message = tr("アップデータを起動できませんでした。");
        setError(message);
        showUpdateError(tr("更新に失敗しました"), message);
        return false;
    }
    setError({});
    return true;
}

void Backend::cancelUpdateDownload() {
    m_updateCancelled = true;
    if (m_updateReply) {
        m_updateReply->abort();
    }
}

void Backend::showUpdateError(const QString &title, const QString &text) {
#ifdef Q_OS_WIN
    MessageBoxW(GetActiveWindow(),
                reinterpret_cast<LPCWSTR>(text.utf16()),
                reinterpret_cast<LPCWSTR>(title.utf16()),
                MB_OK | MB_ICONWARNING);
#endif
}

void Backend::initialize() {
    if (m_handle) {
        UtauTTSDestroy(m_handle);
        m_handle = 0;
        emit connectedChanged();
    }
    const QDir root = resourceRoot();
    QJsonObject config{{"voice_dir", root.filePath("voice")}};
    QJsonArray rendererDirectories{root.filePath("plugins/renderers")};
    if (QDir(externalRendererRootPath()).absolutePath()
            != QDir(root.filePath("plugins/renderers")).absolutePath())
        rendererDirectories.append(externalRendererRootPath());
    config.insert("renderer_directories", rendererDirectories);
    config.insert("model_directories", QJsonArray{root.filePath("models")});
    const QString runtime = root.filePath("runtime");
#ifdef Q_OS_WIN
    const QString openJTalkName = QStringLiteral("utautts-openjtalk-features.exe");
#else
    const QString openJTalkName = QStringLiteral("utautts-openjtalk-features");
#endif
    const QString openJTalkPath = QDir(runtime).filePath(openJTalkName);
    const QString openJTalkDictionary = QDir(runtime).filePath("open_jtalk_dic_utf_8-1.11");
    if (QFileInfo(openJTalkPath).isFile()) {
        config.insert("openjtalk_path", openJTalkPath);
    }
    if (QFileInfo(openJTalkDictionary).isDir()) {
        config.insert("openjtalk_dictionary", openJTalkDictionary);
    }
    QByteArray encoded = QJsonDocument(config).toJson(QJsonDocument::Compact);
    m_handle = UtauTTSCreate(encoded.data());
    if (!m_handle) {
        std::unique_ptr<char, decltype(&UtauTTSFree)> detail(UtauTTSLastError(), &UtauTTSFree);
        const QString message = detail ? QString::fromUtf8(detail.get()) : QString();
        setError(message.isEmpty() ? tr("Goバックエンドを初期化できませんでした") : message);
        return;
    }
    emit connectedChanged();
    try { refreshMetadata(); setError({}); } catch (const std::exception &exception) { setError(QString::fromUtf8(exception.what())); }
}

bool Backend::restartNativeBackend() {
    if (m_busy || m_activeCallCount != 0) {
        setError(tr("処理中はRendererを変更できません。"));
        return false;
    }
    clearPreviewCache();
    initialize();
    return m_handle != 0;
}

QString Backend::addExternalRenderer(const QUrl &executable) {
    if (m_busy || m_activeCallCount != 0) {
        setError(tr("処理中はRendererを変更できません。"));
        return {};
    }
    const QString path = QFileInfo(executable.toLocalFile()).absoluteFilePath();
    const QFileInfo info(path);
    if (!info.isFile()) {
        setError(tr("Rendererの実行ファイルが見つかりません。"));
        return {};
    }
#ifdef Q_OS_WIN
    if (info.suffix().compare(QStringLiteral("exe"), Qt::CaseInsensitive) != 0) {
        setError(tr("Windowsでは.exe形式のUTAU Rendererを指定してください。"));
        return {};
    }
#else
    if (!info.isExecutable()) {
        setError(tr("Rendererの実行権限がありません。"));
        return {};
    }
#endif
    QString slug = info.completeBaseName().toLower();
    slug.replace(QRegularExpression(QStringLiteral("[^a-z0-9]+")), QStringLiteral("-"));
    slug = slug.trimmed();
    slug.remove(QRegularExpression(QStringLiteral("^-+|-+$")));
    if (slug.isEmpty())
        slug = QStringLiteral("renderer");
    const QString digest = QString::fromLatin1(
        QCryptographicHash::hash(path.toUtf8(), QCryptographicHash::Sha256).toHex().left(12));
    const QString id = QStringLiteral("utau-external-%1-%2").arg(slug, digest);
    const QString manifestPath = QDir(externalRendererRootPath()).filePath(
        QStringLiteral("%1/plugin.json").arg(id));
    const QVariantMap manifest{
        {"manifest_version", 1}, {"kind", "renderer"}, {"id", id},
        {"display_name", info.completeBaseName()},
        {"description", tr("外部UTAU互換Renderer")},
        {"backend", "utau-external-resampler"}, {"version", "1"},
        {"acceleration", "cpu"}, {"default_priority", 0},
        {"capabilities", QVariantMap{{"frame_pitch", true}}},
        {"assets", QVariantMap{{"resampler", path}}},
    };
    QString writeError;
    if (!writeJSONFile(manifestPath, manifest, &writeError)) {
        setError(tr("Renderer設定を書き込めませんでした: %1").arg(writeError));
        return {};
    }
    if (!restartNativeBackend())
        return {};
    const bool installed = std::any_of(m_renderers.cbegin(), m_renderers.cend(), [&id](const QVariant &value) {
        return value.toMap().value(QStringLiteral("id")).toString() == id;
    });
    if (!installed)
        setError(tr("Rendererを読み込めませんでした。"));
    return installed ? id : QString();
}

bool Backend::removeExternalRenderer(const QString &id) {
    if (m_busy || m_activeCallCount != 0) {
        setError(tr("処理中はRendererを変更できません。"));
        return false;
    }
    const auto item = std::find_if(m_renderers.cbegin(), m_renderers.cend(), [&id](const QVariant &value) {
        const QVariantMap renderer = value.toMap();
        return renderer.value(QStringLiteral("id")).toString() == id
                && renderer.value(QStringLiteral("backend")).toString() == QStringLiteral("utau-external-resampler");
    });
    if (item == m_renderers.cend()) {
        setError(tr("削除できる外部Rendererではありません。"));
        return false;
    }
    const QDir root(externalRendererRootPath());
    QDir target(root.filePath(id));
    const QString expectedParent = QFileInfo(target.absolutePath()).absoluteDir().absolutePath();
    const QString manifestPath = target.filePath(QStringLiteral("plugin.json"));
    if (expectedParent != root.absolutePath() || !target.exists() || !QFile::remove(manifestPath)) {
        setError(tr("Renderer設定を削除できませんでした。"));
        return false;
    }
    root.rmdir(id);
    return restartNativeBackend();
}

QVariantMap Backend::call(const QByteArray &method, const QVariantMap &request) {
    if (!m_handle) {
        throw std::runtime_error("backend is not initialized");
    }
    QByteArray methodCopy = method;
    QByteArray requestJSON = QJsonDocument::fromVariant(request).toJson(QJsonDocument::Compact);
    std::unique_ptr<char, decltype(&UtauTTSFree)> response(
        UtauTTSCall(m_handle, methodCopy.data(), requestJSON.data()), &UtauTTSFree);
    if (!response) {
        throw std::runtime_error("native backend returned no response");
    }
    QJsonParseError parseError;
    const QJsonDocument document = QJsonDocument::fromJson(response.get(), &parseError);
    if (parseError.error != QJsonParseError::NoError || !document.isObject()) {
        throw std::runtime_error("native backend returned invalid JSON");
    }
    const QJsonObject object = document.object();
    if (!object.value("ok").toBool()) {
        throw std::runtime_error(object.value("error").toString().toStdString());
    }
    const QJsonValue result = object.value("result");
    if (!result.isObject()) {
        throw std::runtime_error("native backend returned no result");
    }
    return result.toObject().toVariantMap();
}

void Backend::refreshMetadata() {
    const QVariantMap voices = call("voicebanks");
    const QVariantMap models = call("models");
    const QVariantMap renderers = call("renderers");
    m_voicebanks = voices.value("voicebanks").toList();
    m_models = models.value("models").toList();
    m_renderers = renderers.value("renderers").toList();
    m_catalogDefaultRenderer = renderers.value("default_renderer").toString();
    const auto containsId = [](const QVariantList &items, const QString &id) {
        return std::any_of(items.cbegin(), items.cend(), [&id](const QVariant &item) {
            return item.toMap().value(QStringLiteral("id")).toString() == id;
        });
    };
    if ((m_defaultRenderer == QLatin1String("openutau-classic-worldline-faithful")
            || m_defaultRenderer == QLatin1String("openutau-classic-worldline-faithful-gpu"))
            && containsId(m_renderers, QStringLiteral("openutau-worldline-r-faithful")))
        m_defaultRenderer = QStringLiteral("openutau-worldline-r-faithful");
    else if (!containsId(m_renderers, m_defaultRenderer))
        m_defaultRenderer = m_catalogDefaultRenderer;
    if (m_defaultModelId != QStringLiteral("none") && !containsId(m_models, m_defaultModelId))
        m_defaultModelId = m_models.isEmpty()
                ? QStringLiteral("none") : m_models.constFirst().toMap().value(QStringLiteral("id")).toString();
    emit synthesisDefaultsChanged();
    emit metadataChanged();
}

void Backend::reloadVoicebanks() {
    if (m_busy) {
        return;
    }
    setBusy(true);
    setError({});
    auto *watcher = new QFutureWatcher<QVariantMap>(this);
    connect(watcher, &QFutureWatcher<QVariantMap>::finished, this, [this, watcher]() {
        setBusy(false);
        const QVariantMap result = watcher->result();
        if (result.contains("_error")) {
            setError(result.value("_error").toString());
        } else {
            clearPreviewCache();
            m_voicebanks = result.value("voicebanks").toList();
            emit metadataChanged();
        }
        watcher->deleteLater();
        if (--m_activeCallCount == 0) {
            m_activeCalls.clearFutures();
        }
    });
    const auto future = QtConcurrent::run([this]() {
        try {
            return call("reloadVoicebanks");
        } catch (const std::exception &exception) {
            return QVariantMap{{"_error", QString::fromUtf8(exception.what())}};
        }
    });
    ++m_activeCallCount;
    m_activeCalls.addFuture(future);
    watcher->setFuture(future);
}

bool Backend::openVoiceDirectory() {
    const QDir voiceDirectory(resourceRoot().filePath(QStringLiteral("voice")));
    if (!voiceDirectory.exists() && !QDir().mkpath(voiceDirectory.absolutePath())) {
        setError(QStringLiteral("Failed to create the voice directory."));
        return false;
    }
    const bool opened = QDesktopServices::openUrl(QUrl::fromLocalFile(voiceDirectory.absolutePath()));
    if (!opened) {
        setError(QStringLiteral("Failed to open the voice directory."));
        return false;
    }
    setError({});
    return true;
}

void Backend::analyze(const QString &text, const QString &requestId) {
    if (m_busy) {
        return;
    }
    const quint64 generation = ++m_nextAnalysisGeneration;
    m_analysisGenerations.insert(requestId, generation);
    auto *watcher = new QFutureWatcher<QVariantMap>(this);
    connect(watcher, &QFutureWatcher<QVariantMap>::finished, this,
            [this, watcher, generation, requestId, text]() {
                const QVariantMap value = watcher->result();
                if (m_analysisGenerations.value(requestId) == generation) {
                    m_analysisGenerations.remove(requestId);
                    if (value.contains("_error")) {
                        setError(value.value("_error").toString());
                    } else {
                        m_analysisRequestId = requestId;
                        m_analysisSourceText = text;
                        m_analysisJson = QString::fromUtf8(
                            QJsonDocument::fromVariant(value).toJson(QJsonDocument::Compact));
                        emit analysisChanged();
                        setError({});
                    }
                }
                watcher->deleteLater();
                if (--m_activeCallCount == 0) {
                    m_activeCalls.clearFutures();
                }
            });
    const QVariantList dictionary = m_dictionaryEntries;
    const auto future = QtConcurrent::run([this, text, dictionary]() {
        try {
            return call("analyze", {{"text", text}, {"dictionary", dictionary}});
        } catch (const std::exception &exception) {
            return QVariantMap{{"_error", QString::fromUtf8(exception.what())}};
        }
    });
    ++m_activeCallCount;
    m_activeCalls.addFuture(future);
    watcher->setFuture(future);
}

void Backend::predictProsody(const QVariantMap &request) {
    if (m_busy) {
        return;
    }
    QString requestId = request.value("request_id").toString();
    if (requestId.isEmpty()) {
        requestId = QUuid::createUuid().toString(QUuid::WithoutBraces);
    }
    const quint64 generation = ++m_nextProsodyGeneration;
    auto *watcher = new QFutureWatcher<QVariantMap>(this);
    connect(watcher, &QFutureWatcher<QVariantMap>::finished, this,
            [this, watcher, generation, requestId]() {
                const QVariantMap value = watcher->result();
                if (generation == m_nextProsodyGeneration) {
                    if (value.contains("_error")) {
                        setError(value.value("_error").toString());
                    } else {
                        m_prosodyRequestId = requestId;
                        m_prosodyJson = QString::fromUtf8(
                            QJsonDocument::fromVariant(value).toJson(QJsonDocument::Compact));
                        emit prosodyChanged();
                        setError({});
                    }
                }
                watcher->deleteLater();
                if (--m_activeCallCount == 0) {
                    m_activeCalls.clearFutures();
                }
            });
    QVariantMap callRequest = request;
    callRequest.insert("request_id", requestId);
    const auto future = QtConcurrent::run([this, callRequest]() {
        try {
            return call("predictProsody", callRequest);
        } catch (const std::exception &exception) {
            return QVariantMap{{"_error", QString::fromUtf8(exception.what())}};
        }
    });
    ++m_activeCallCount;
    m_activeCalls.addFuture(future);
    watcher->setFuture(future);
}

void Backend::synthesize(const QVariantMap &input) {
    if (m_busy) {
        return;
    }
    if (!m_previewDirectory.isValid()) {
        setError(tr("プレビュー用の一時ディレクトリを作成できませんでした"));
        return;
    }
    QVariantMap request = input;
    request.remove(QStringLiteral("output_path"));
    const QByteArray cacheKey = previewCacheKey(request);
    if (restorePreviewCache(cacheKey)) {
        return;
    }
    const QString previewText = request.value("text").toString().isEmpty()
            ? request.value("kana").toString() : request.value("text").toString();
    const QString outputPath = m_previewDirectory.filePath(
        "utautts-" + QUuid::createUuid().toString(QUuid::WithoutBraces) + ".wav");
    request.insert("output_path", outputPath);
    appendLog(tr("音声合成を開始しました: %1").arg(request.value("text").toString()));
    setBusy(true);
    setError({});
    auto *watcher = new QFutureWatcher<QVariantMap>(this);
    connect(watcher, &QFutureWatcher<QVariantMap>::finished, this,
            [this, watcher, outputPath, previewText, cacheKey]() {
                setBusy(false);
                const QVariantMap result = watcher->result();
                if (result.contains("_error")) {
                    const QString error = result.value("_error").toString();
                    appendLog(tr("音声合成に失敗しました: %1").arg(error));
                    setError(error);
                } else {
                    m_previewPath = outputPath;
                    m_previewText = previewText;
                    m_previewLab = result.value("lab").toString();
                    m_previewUrl = QUrl::fromLocalFile(outputPath);
                    m_synthesisJson = QString::fromUtf8(
                        QJsonDocument::fromVariant(result).toJson(QJsonDocument::Compact));
                    storePreviewCache(cacheKey, PreviewCacheEntry{
                        outputPath, previewText, m_previewLab, m_synthesisJson});
                    emit synthesisChanged();
                    appendLog(tr("音声合成が完了しました。"));
                    emit previewReady();
                }
                watcher->deleteLater();
                if (--m_activeCallCount == 0) {
                    m_activeCalls.clearFutures();
                }
            });
    const auto future = QtConcurrent::run([this, request]() {
        try {
            return call("synthesize", request);
        } catch (const std::exception &exception) {
            return QVariantMap{{"_error", QString::fromUtf8(exception.what())}};
        }
    });
    ++m_activeCallCount;
    m_activeCalls.addFuture(future);
    watcher->setFuture(future);
}

QByteArray Backend::previewCacheKey(const QVariantMap &request) const {
    return QCryptographicHash::hash(
        QJsonDocument::fromVariant(request).toJson(QJsonDocument::Compact),
        QCryptographicHash::Sha256);
}

bool Backend::restorePreviewCache(const QByteArray &key) {
    const auto found = m_previewCache.constFind(key);
    if (found == m_previewCache.cend())
        return false;
    const PreviewCacheEntry entry = found.value();
    if (!QFileInfo::exists(entry.path)) {
        m_previewCache.remove(key);
        m_previewCacheOrder.removeAll(key);
        return false;
    }
    m_previewCacheOrder.removeAll(key);
    m_previewCacheOrder.append(key);
    m_previewPath = entry.path;
    m_previewText = entry.text;
    m_previewLab = entry.lab;
    m_previewUrl = QUrl::fromLocalFile(entry.path);
    m_synthesisJson = entry.synthesisJson;
    setError({});
    appendLog(tr("キャッシュ済みの音声を使用しました。"));
    emit synthesisChanged();
    QMetaObject::invokeMethod(this, [this] { emit previewReady(); }, Qt::QueuedConnection);
    return true;
}

void Backend::storePreviewCache(const QByteArray &key, const PreviewCacheEntry &entry) {
    const auto existing = m_previewCache.constFind(key);
    if (existing != m_previewCache.cend() && existing->path != entry.path)
        QFile::remove(existing->path);
    m_previewCache.insert(key, entry);
    m_previewCacheOrder.removeAll(key);
    m_previewCacheOrder.append(key);
    trimPreviewCache();
}

void Backend::trimPreviewCache() {
    while (m_previewCacheOrder.size() > m_previewCacheFileCount) {
        const QByteArray oldest = m_previewCacheOrder.takeFirst();
        const PreviewCacheEntry entry = m_previewCache.take(oldest);
        if (!entry.path.isEmpty() && entry.path != m_previewPath)
            QFile::remove(entry.path);
    }
}

void Backend::clearPreviewCache() {
    for (auto iterator = m_previewCache.cbegin(); iterator != m_previewCache.cend(); ++iterator) {
        const PreviewCacheEntry &entry = iterator.value();
        if (!entry.path.isEmpty())
            QFile::remove(entry.path);
    }
    m_previewCache.clear();
    m_previewCacheOrder.clear();
    m_previewPath.clear();
    m_previewText.clear();
    m_previewLab.clear();
    m_previewUrl = QUrl{};
    m_synthesisJson.clear();
    emit synthesisChanged();
}

bool Backend::savePreview(const QUrl &destination) {
    if (m_previewPath.isEmpty() || !destination.isLocalFile()) {
        setError(tr("保存できるプレビュー音声がありません"));
        return false;
    }
    QFile source(m_previewPath);
    QSaveFile target(destination.toLocalFile());
    if (!source.open(QIODevice::ReadOnly) || !target.open(QIODevice::WriteOnly)) {
        setError(tr("WAVの保存先を開けませんでした"));
        return false;
    }
    constexpr qint64 chunkSize = 1024 * 1024;
    while (!source.atEnd()) {
        const QByteArray chunk = source.read(chunkSize);
        if (chunk.isEmpty() && source.error() != QFileDevice::NoError) {
            target.cancelWriting();
            setError(tr("プレビューWAVを読み込めませんでした"));
            return false;
        }
        if (target.write(chunk) != chunk.size()) {
            target.cancelWriting();
            setError(tr("WAVを保存できませんでした"));
            return false;
        }
    }
    if (!target.commit()) {
        setError(tr("WAVを保存できませんでした"));
        return false;
    }
    if (m_exportTextWithWav || m_exportLabWithWav) {
        try {
            call("writeSidecars", QVariantMap{
                {"wav_path", destination.toLocalFile()},
                {"text", m_previewText},
                {"lab", m_previewLab},
                {"encoding", m_exportTextEncoding},
                {"write_text", m_exportTextWithWav},
                {"write_lab", m_exportLabWithWav},
            });
        } catch (const std::exception &exception) {
            setError(tr("付随するTXT／LABファイルを保存できませんでした: %1")
                         .arg(QString::fromUtf8(exception.what())));
            return false;
        }
    }
    setError({});
    return true;
}

bool Backend::startFileDrag(const QVariantList &files) {
    QList<QUrl> urls;
    urls.reserve(files.size());
    for (const QVariant &value : files) {
        const QUrl url = value.canConvert<QUrl>() ? value.toUrl() : QUrl(value.toString());
        if (!url.isLocalFile() || url.toLocalFile().isEmpty() || !QFileInfo::exists(url.toLocalFile())) {
            setError(tr("ドラッグするWAVファイルが見つかりません"));
            return false;
        }
        urls.append(QUrl::fromLocalFile(QFileInfo(url.toLocalFile()).absoluteFilePath()));
    }
    if (urls.isEmpty()) {
        setError(tr("ドラッグするWAVファイルがありません"));
        return false;
    }

    auto *mimeData = new QMimeData;
    mimeData->setUrls(urls);
    auto *drag = new QDrag(this);
    drag->setMimeData(mimeData);
    drag->exec(Qt::CopyAction);
    setError({});
    return true;
}

QUrl Backend::writeDragExo(const QUrl &directory, const QVariantList &files, int frameRate) {
    if (!directory.isLocalFile() || files.isEmpty()) {
        setError(tr("ドラッグ用のexoファイルを作成できません"));
        return {};
    }
    QStringList paths;
    paths.reserve(files.size());
    for (const QVariant &value : files) {
        const QUrl url = value.canConvert<QUrl>() ? value.toUrl() : QUrl(value.toString());
        if (!url.isLocalFile() || url.toLocalFile().isEmpty() || !QFileInfo::exists(url.toLocalFile())) {
            setError(tr("ドラッグするWAVファイルが見つかりません"));
            return {};
        }
        paths.append(QFileInfo(url.toLocalFile()).absoluteFilePath());
    }
    const int boundedFrameRate = qBound(1, frameRate, 240);
    const QString exoPath = QDir(directory.toLocalFile()).filePath(QStringLiteral("utautts.exo"));
    const QVariantMap request{{"output_path", QDir::toNativeSeparators(exoPath)}, {"files", paths}, {"frame_rate", boundedFrameRate}};
    try {
        call("writeExo", request);
    } catch (const std::exception &exception) {
        setError(QString::fromUtf8(exception.what()));
        return {};
    }
    setError({});
    return QUrl::fromLocalFile(exoPath);
}

QUrl Backend::defaultSaveFile(const QString &fileName) const {
    const QFileInfo fileInfo(fileName);
    if (fileName.isEmpty() || fileInfo.fileName() != fileName) {
        return {};
    }
    QString directoryPath = QStandardPaths::writableLocation(QStandardPaths::DocumentsLocation);
    if (directoryPath.isEmpty()) {
        directoryPath = QDir::homePath();
    }
    return QUrl::fromLocalFile(QDir(directoryPath).filePath(fileName));
}

QUrl Backend::fileInDirectory(const QUrl &directory, const QString &fileName) const {
    const QFileInfo fileInfo(fileName);
    if (!directory.isLocalFile() || fileName.isEmpty() || fileInfo.fileName() != fileName) {
        return {};
    }
    return QUrl::fromLocalFile(QDir(directory.toLocalFile()).filePath(fileName));
}

bool Backend::saveProject(const QUrl &destination, const QVariantMap &project) {
    if (!destination.isLocalFile()) {
        setError(tr("プロジェクトの保存先が無効です"));
        return false;
    }
    const QJsonDocument document = QJsonDocument::fromVariant(project);
    if (!document.isObject()) {
        setError(tr("プロジェクトのデータが無効です"));
        return false;
    }
    const QByteArray data = document.toJson(QJsonDocument::Indented);
    QSaveFile target(destination.toLocalFile());
    if (!target.open(QIODevice::WriteOnly) || target.write(data) != data.size() || !target.commit()) {
        target.cancelWriting();
        setError(tr("プロジェクトを保存できませんでした"));
        return false;
    }
    setError({});
    return true;
}

void Backend::exportUstx(const QUrl &destination, const QVariantMap &project) {
    if (!destination.isLocalFile()) {
        emit ustxExportFinished(false, tr("USTXの保存先が無効です"));
        return;
    }
    if (m_busy) {
        return;
    }
    setBusy(true);
    setError({});
    const QString outputPath = QDir::toNativeSeparators(destination.toLocalFile());
    const QVariantMap request{{"output_path", outputPath}, {"project", project}};
    auto *watcher = new QFutureWatcher<QVariantMap>(this);
    connect(watcher, &QFutureWatcher<QVariantMap>::finished, this, [this, watcher, outputPath]() {
        setBusy(false);
        const QVariantMap result = watcher->result();
        if (result.contains("_error")) {
            const QString error = result.value("_error").toString();
            setError(error);
            emit ustxExportFinished(false, error);
        } else {
            emit ustxExportFinished(true, outputPath);
        }
        watcher->deleteLater();
        if (--m_activeCallCount == 0) {
            m_activeCalls.clearFutures();
        }
    });
    const auto future = QtConcurrent::run([this, request]() {
        try {
            call("exportUstx", request);
            return QVariantMap();
        } catch (const std::exception &exception) {
            return QVariantMap{{"_error", QString::fromUtf8(exception.what())}};
        }
    });
    ++m_activeCallCount;
    m_activeCalls.addFuture(future);
    watcher->setFuture(future);
}

QVariantMap Backend::loadProject(const QUrl &source) {
    if (!source.isLocalFile()) {
        setError(tr("プロジェクトファイルが無効です"));
        return {{"_error", error()}};
    }
    QFile file(source.toLocalFile());
    if (!file.open(QIODevice::ReadOnly)) {
        setError(tr("プロジェクトファイルを開けませんでした"));
        return {{"_error", error()}};
    }
    QJsonParseError parseError;
    const QJsonDocument document = QJsonDocument::fromJson(file.readAll(), &parseError);
    if (parseError.error != QJsonParseError::NoError || !document.isObject()) {
        setError(tr("プロジェクトファイルの形式が正しくありません"));
        return {{"_error", error()}};
    }
    const QVariantMap project = document.toVariant().toMap();
    const QVariantList utterances = project.value("utterances").toList();
    if (project.value("format").toString() != "utautts-project"
            || project.value("format_version").toInt() < 1 || !project.contains("utterances")) {
        setError(tr("対応していないプロジェクト形式です"));
        return {{"_error", error()}};
    }
    Q_UNUSED(utterances)
    setError({});
    return project;
}

void Backend::rememberRecentProject(const QUrl &source) {
    if (!source.isLocalFile())
        return;
    const QString path = QFileInfo(source.toLocalFile()).absoluteFilePath();
    if (path.isEmpty())
        return;

    QStringList updated{path};
    for (const QString &existing : m_recentProjects) {
        const QString absolutePath = QFileInfo(existing).absoluteFilePath();
        if (!QFileInfo(absolutePath).isFile() || absolutePath == path || updated.size() >= maxRecentProjects)
            continue;
        updated.append(absolutePath);
    }
    if (m_recentProjects == updated)
        return;
    m_recentProjects = updated;
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.setValue(QStringLiteral("projects/recent"), m_recentProjects);
    settings.sync();
    emit recentProjectsChanged();
}

void Backend::removeRecentProject(const QString &path) {
    const QString absolutePath = QFileInfo(path).absoluteFilePath();
    QStringList updated;
    for (const QString &existing : m_recentProjects) {
        if (QFileInfo(existing).absoluteFilePath() != absolutePath)
            updated.append(existing);
    }
    if (updated == m_recentProjects)
        return;
    m_recentProjects = updated;
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.setValue(QStringLiteral("projects/recent"), m_recentProjects);
    settings.sync();
    emit recentProjectsChanged();
}

void Backend::clearRecentProjects() {
    if (m_recentProjects.isEmpty())
        return;
    m_recentProjects.clear();
    QSettings settings(portableSettingsPath(), QSettings::IniFormat);
    settings.remove(QStringLiteral("projects/recent"));
    settings.sync();
    emit recentProjectsChanged();
}

bool Backend::exportDiagnosticReport(const QUrl &destination, const QVariantMap &context) {
    if (!destination.isLocalFile()) {
        setError(tr("診断情報の保存先が無効です"));
        return false;
    }

    QVariantList voicebanks;
    for (const QVariant &value : m_voicebanks) {
        const QVariantMap source = value.toMap();
        voicebanks.append(QVariantMap{
            {"id", source.value("id")},
            {"name", source.value("name")},
            {"alias_counts", source.value("alias_counts")},
            {"vcv_contexts", source.value("vcv_contexts")},
            {"vc_contexts", source.value("vc_contexts")},
            {"has_vc", source.value("has_vc")},
            {"has_initial_vcv", source.value("has_initial_vcv")},
            {"has_n_context_vcv", source.value("has_n_context_vcv")},
        });
    }

    QVariantList models;
    for (const QVariant &value : m_models) {
        const QVariantMap source = value.toMap();
        models.append(QVariantMap{
            {"id", source.value("id")},
            {"display_name", source.value("display_name")},
            {"version", source.value("version")},
            {"format", source.value("format")},
            {"license", source.value("license")},
            {"recommended_renderers", source.value("recommended_renderers")},
        });
    }

    QVariantList renderers;
    for (const QVariant &value : m_renderers) {
        const QVariantMap source = value.toMap();
        renderers.append(QVariantMap{
            {"id", source.value("id")},
            {"display_name", source.value("display_name")},
            {"version", source.value("version")},
            {"backend", source.value("backend")},
            {"acceleration", source.value("acceleration")},
            {"experimental", source.value("experimental")},
            {"capabilities", source.value("capabilities")},
        });
    }

    const QString homePath = QDir::toNativeSeparators(QDir::homePath());
    const QString applicationPath = QDir::toNativeSeparators(resourceRoot().absolutePath());
    QStringList logs;
    const QRegularExpression synthesisText(
        QStringLiteral("^(\\[[^\\]]+\\]\\s+音声合成を開始しました:\\s*).*$"));
    for (QString line : m_logLines) {
        line.replace(synthesisText, QStringLiteral("\\1<redacted>"));
        if (!homePath.isEmpty())
            line.replace(homePath, QStringLiteral("<home>"), Qt::CaseInsensitive);
        if (!applicationPath.isEmpty())
            line.replace(applicationPath, QStringLiteral("<application>"), Qt::CaseInsensitive);
        logs.append(line);
    }

    const QStringList contextKeys{
        QStringLiteral("voicebank_id"), QStringLiteral("model_id"), QStringLiteral("renderer"),
        QStringLiteral("alias_policy"), QStringLiteral("tone"), QStringLiteral("color"),
        QStringLiteral("mora_duration_ms"), QStringLiteral("pause_duration_ms"),
        QStringLiteral("intonation_strength"), QStringLiteral("apply_pitch"),
    };
    QVariantMap selection;
    for (const QString &key : contextKeys) {
        if (context.contains(key))
            selection.insert(key, context.value(key));
    }

    const QVariantMap report{
        {"format", QStringLiteral("utautts-diagnostic-report")},
        {"format_version", 1},
        {"exported_at", QDateTime::currentDateTimeUtc().toString(Qt::ISODateWithMs)},
        {"application", QVariantMap{
            {"name", QCoreApplication::applicationName()},
            {"version", QCoreApplication::applicationVersion()},
        }},
        {"environment", QVariantMap{
            {"qt_version", QString::fromLatin1(qVersion())},
            {"os", QSysInfo::prettyProductName()},
            {"kernel_type", QSysInfo::kernelType()},
            {"kernel_version", QSysInfo::kernelVersion()},
            {"cpu_architecture", QSysInfo::currentCpuArchitecture()},
            {"build_abi", QSysInfo::buildAbi()},
            {"locale", QLocale::system().name()},
        }},
        {"settings", QVariantMap{
            {"language", m_language},
            {"resolved_language", resolvedLanguage()},
            {"dark_mode", m_darkMode},
            {"default_voicebank_id", m_defaultVoicebankId},
            {"default_model_id", m_defaultModelId},
            {"default_renderer_id", m_defaultRenderer},
            {"default_mora_duration_ms", m_defaultMoraDuration},
            {"default_pause_duration_ms", m_defaultPauseDuration},
            {"default_leading_preutterance_ms", m_defaultLeadingPreutterance},
            {"default_intonation_strength", m_defaultIntonationStrength},
            {"default_tone", m_defaultTone},
            {"default_alias_policy", m_defaultAliasPolicy},
            {"export_text_with_wav", m_exportTextWithWav},
            {"export_lab_with_wav", m_exportLabWithWav},
            {"export_text_encoding", m_exportTextEncoding},
            {"close_log_on_success", m_closeLogOnSuccess},
            {"update_check_enabled", m_updateCheckEnabled},
            {"developer_mode", m_developerMode},
        }},
        {"current_selection", selection},
        {"catalog", QVariantMap{
            {"default_renderer", m_catalogDefaultRenderer},
            {"voicebanks", voicebanks},
            {"models", models},
            {"renderers", renderers},
        }},
        {"logs", logs},
        {"privacy", QVariantMap{
            {"includes_input_text", false},
            {"includes_audio", false},
            {"includes_absolute_voicebank_paths", false},
        }},
    };

    QString writeError;
    if (!writeJSONFile(destination.toLocalFile(), report, &writeError)) {
        setError(tr("診断情報を書き出せませんでした: %1").arg(writeError));
        return false;
    }
    setError({});
    return true;
}

QVariantMap Backend::loadProsodyPromptSet() const {
    QFile file(QStringLiteral(":/data/prosody-prompts-ja-v1.json"));
    if (!file.open(QIODevice::ReadOnly))
        return {{"_error", tr("教師データ用の文章セットを開けませんでした")}};
    QJsonParseError parseError;
    const QJsonDocument document = QJsonDocument::fromJson(file.readAll(), &parseError);
    if (parseError.error != QJsonParseError::NoError || !document.isObject())
        return {{"_error", tr("教師データ用の文章セットが壊れています")}};
    return document.toVariant().toMap();
}

QVariantMap Backend::loadProsodyTrainingSession() {
    QFile file(prosodyTrainingSessionPath());
    if (!file.exists())
        return {};
    if (!file.open(QIODevice::ReadOnly)) {
        setError(tr("教師データ収集の途中保存を開けませんでした"));
        return {{"_error", error()}};
    }
    QJsonParseError parseError;
    const QJsonDocument document = QJsonDocument::fromJson(file.readAll(), &parseError);
    if (parseError.error != QJsonParseError::NoError || !document.isObject()) {
        setError(tr("教師データ収集の途中保存が壊れています"));
        return {{"_error", error()}};
    }
    setError({});
    return document.toVariant().toMap();
}

bool Backend::saveProsodyTrainingSession(const QVariantMap &session) {
    if (session.value("format").toString() != QLatin1String("utautts-prosody-training-session")
            || session.value("format_version").toInt() != 1) {
        setError(tr("教師データ収集の保存内容が無効です"));
        return false;
    }
    QString writeError;
    if (!writeJSONFile(prosodyTrainingSessionPath(), session, &writeError)) {
        setError(tr("教師データ収集を途中保存できませんでした: %1").arg(writeError));
        return false;
    }
    setError({});
    return true;
}

bool Backend::clearProsodyTrainingSession() {
    const QString path = prosodyTrainingSessionPath();
    if (QFileInfo::exists(path) && !QFile::remove(path)) {
        setError(tr("教師データ収集の途中保存を削除できませんでした"));
        return false;
    }
    setError({});
    return true;
}

bool Backend::exportProsodyTrainingDataset(const QUrl &destination, const QVariantMap &session) {
    if (!destination.isLocalFile()) {
        setError(tr("教師データの保存先が無効です"));
        return false;
    }
    const QVariantList records = session.value("records").toList();
    QByteArray jsonl;
    int accepted = 0, skipped = 0, drafts = 0;
    for (const QVariant &value : records) {
        const QVariantMap record = value.toMap();
        if (!record.value("accepted").toBool()) {
            if (record.value("status").toString() == QLatin1String("skipped"))
                ++skipped;
            else if (record.value("status").toString() == QLatin1String("draft"))
                ++drafts;
            continue;
        }
        const int moraCount = record.value("morae").toList().size();
        if (record.value("text").toString().isEmpty()
                || record.value("reading").toString().isEmpty()
                || moraCount == 0
                || record.value("features").toList().size() != moraCount
                || record.value("base_points_cents").toList().size() != moraCount
                || record.value("manual_offsets_cents").toList().size() != moraCount
                || record.value("edit_mask").toList().size() != moraCount) {
            setError(tr("教師データに不完全な発話が含まれています"));
            return false;
        }
        const QJsonDocument document = QJsonDocument::fromVariant(record);
        if (!document.isObject()) {
            setError(tr("教師データに無効な発話が含まれています"));
            return false;
        }
        jsonl += document.toJson(QJsonDocument::Compact);
        jsonl += '\n';
        ++accepted;
    }
    if (accepted == 0) {
        setError(tr("書き出せる確認済み発話がありません"));
        return false;
    }

    QSaveFile dataset(destination.toLocalFile());
    if (!dataset.open(QIODevice::WriteOnly) || dataset.write(jsonl) != jsonl.size() || !dataset.commit()) {
        dataset.cancelWriting();
        setError(tr("教師データを書き出せませんでした"));
        return false;
    }

    QFileInfo destinationInfo(destination.toLocalFile());
    const QString baseName = destinationInfo.completeBaseName();
    const QString reportPath = destinationInfo.dir().filePath(baseName + QStringLiteral("-report.json"));
    QVariantMap report{
        {"format", QStringLiteral("utautts-prosody-training-report")},
        {"format_version", 1},
        {"session_id", session.value("session_id")},
        {"shuffle_seed", session.value("shuffle_seed")},
        {"prompt_set", session.value("prompt_set")},
        {"synthesis_context", session.value("synthesis_context")},
        {"accepted_count", accepted},
        {"record_count", records.size()},
        {"skipped_count", skipped},
        {"draft_count", drafts},
        {"exported_at", QDateTime::currentDateTimeUtc().toString(Qt::ISODateWithMs)},
    };
    QString writeError;
    if (!writeJSONFile(reportPath, report, &writeError)) {
        setError(tr("教師データのレポートを書き出せませんでした: %1").arg(writeError));
        return false;
    }
    setError({});
    return true;
}

QString Backend::dictionaryFingerprint() const {
    const QByteArray data = QJsonDocument::fromVariant(m_dictionaryEntries).toJson(QJsonDocument::Compact);
    return QString::fromLatin1(QCryptographicHash::hash(data, QCryptographicHash::Sha256).toHex());
}

void Backend::setBusy(bool value) {
    if (m_busy == value) {
        return;
    }
    m_busy = value;
    emit busyChanged();
}

void Backend::setError(const QString &value) {
    if (m_error == value) {
        return;
    }
    m_error = value;
    emit errorChanged();
}
