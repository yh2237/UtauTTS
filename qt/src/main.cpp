#include "backend.h"
#include "selftest.h"
#include <QDir>
#include <QDateTime>
#include <QFile>
#include <QFileInfo>
#include <QGuiApplication>
#include <QIcon>
#include <QJsonDocument>
#include <QJsonObject>
#include <QQmlApplicationEngine>
#include <QQuickStyle>
#include <QUrl>
#include <QVariantList>
#include <QStandardPaths>
#include <QTemporaryDir>
#include <memory>

#ifdef Q_OS_WIN
#include <windows.h>
#else
#include <cerrno>
#include <signal.h>
#include <sys/types.h>
#endif

namespace {
QString readTextResource(const QString &path) {
    QFile file(path);
    if (!file.open(QIODevice::ReadOnly | QIODevice::Text)) {
        return {};
    }
    return QString::fromUtf8(file.readAll()).trimmed();
}

QVariantList legalDocuments() {
    QVariantList documents;
    documents.append(QVariantMap{{"name", "UtauTTS"},
                                 {"text", readTextResource(":/legal/LICENSE")}});

    const QString notices = readTextResource(":/legal/THIRD_PARTY_NOTICES.txt");
    const QStringList lines = notices.split('\n');
    QString sectionName;
    int contentStart = 0;
    for (int index = 1; index < lines.size(); ++index) {
        const QString underline = lines.at(index).trimmed();
        if (underline.size() < 3 || underline.count('=') != underline.size()) {
            continue;
        }
        const int headingIndex = index - 1;
        if (sectionName.isEmpty()) {
            sectionName = lines.at(headingIndex).trimmed();
            contentStart = index + 1;
            continue;
        }
        const QString text = lines.mid(contentStart, headingIndex - contentStart).join('\n').trimmed();
        if (!text.isEmpty()) {
            documents.append(QVariantMap{{"name", sectionName}, {"text", text}});
        }
        sectionName = lines.at(headingIndex).trimmed();
        contentStart = index + 1;
    }
    if (!sectionName.isEmpty()) {
        const QString finalSection = lines.mid(contentStart).join('\n').trimmed();
        if (!finalSection.isEmpty()) {
            documents.append(QVariantMap{{"name", sectionName}, {"text", finalSection}});
        }
    }
#ifdef Q_OS_WIN
    const QString windowsGuiAddendum =
        readTextResource(":/legal/THIRD_PARTY_NOTICES-WINDOWS-GUI.txt");
    if (!windowsGuiAddendum.isEmpty()) {
        documents.append(QVariantMap{{"name", "Windows GUI third-party addendum"},
                                     {"text", windowsGuiAddendum}});
    }
#endif
    return documents;
}

bool updateProcessAlive(qint64 pid) {
    if (pid <= 0)
        return false;
#ifdef Q_OS_WIN
    HANDLE process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE,
                                 static_cast<DWORD>(pid));
    if (!process)
        return false;
    DWORD exitCode = 0;
    const bool alive = GetExitCodeProcess(process, &exitCode) && exitCode == STILL_ACTIVE;
    CloseHandle(process);
    return alive;
#else
    if (::kill(static_cast<pid_t>(pid), 0) == 0)
        return true;
    return errno == EPERM;
#endif
}

bool updateLockActive(const QFileInfo &info) {
    QFile file(info.absoluteFilePath());
    if (!file.open(QIODevice::ReadOnly | QIODevice::Text))
        return info.lastModified().msecsTo(QDateTime::currentDateTimeUtc()) < 60 * 1000;
    const QJsonDocument document = QJsonDocument::fromJson(file.readAll());
    const QJsonObject state = document.isObject() ? document.object() : QJsonObject{};
    const qint64 pid = state.value(QStringLiteral("updater_pid")).toInteger();
    if (pid > 0 && updateProcessAlive(pid))
        return true;
    QDateTime startedAt = QDateTime::fromString(
        state.value(QStringLiteral("started_at")).toString(), Qt::ISODateWithMs);
    if (!startedAt.isValid())
        startedAt = info.lastModified();
    if (!startedAt.isValid())
        return false;
    const qint64 ageMS = startedAt.toUTC().msecsTo(QDateTime::currentDateTimeUtc());
    return ageMS >= -5000 && ageMS < 60 * 1000;
}

bool updateInProgress() {
    if (qEnvironmentVariable("UTAUTTS_UPDATE_RELAUNCH") == QLatin1String("1"))
        return false;
    QDir root(QCoreApplication::applicationDirPath());
    if (root.dirName().compare(QLatin1String("app"), Qt::CaseInsensitive) == 0)
        root.cdUp();
    const QStringList lockPaths = updateLockPaths(root.absolutePath());
    bool active = false;
    QStringList stalePaths;
    for (const QString &path : lockPaths) {
        if (QFileInfo::exists(path)) {
            const QFileInfo info(path);
            if (updateLockActive(info))
                active = true;
            else
                stalePaths.append(path);
        }
    }
    for (const QString &path : stalePaths)
        QFile::remove(path);
    if (!active)
        return false;
#ifdef Q_OS_WIN
    const QString title = QStringLiteral("UtauTTS 更新中");
    const QString text = QStringLiteral(
        "UtauTTSを更新しています。完了すると自動的に再起動します。\n\n"
        "更新が中断された場合は、インストール先のutautts.exeから起動して復旧してください。");
    MessageBoxW(nullptr, reinterpret_cast<LPCWSTR>(text.utf16()),
                reinterpret_cast<LPCWSTR>(title.utf16()), MB_OK | MB_ICONINFORMATION);
#endif
    return true;
}
}

int main(int argc, char *argv[]) {
    QQuickStyle::setStyle("Fusion");
    QGuiApplication app(argc, argv);
    app.setApplicationName(UTAUTTS_APP_NAME);
    app.setApplicationDisplayName(UTAUTTS_APP_NAME);
    app.setApplicationVersion(UTAUTTS_VERSION);
    app.setOrganizationName(UTAUTTS_APP_ORGANIZATION);

    const bool selfTest = app.arguments().contains(QStringLiteral("--self-test"));
    std::unique_ptr<QTemporaryDir> selfTestSettings;
    if (selfTest) {
        QStandardPaths::setTestModeEnabled(true);
        selfTestSettings = std::make_unique<QTemporaryDir>();
        if (!selfTestSettings->isValid())
            return 2;
        qputenv("UTAUTTS_SELF_TEST_DIRECTORY", selfTestSettings->path().toUtf8());
    }

    if (updateInProgress())
        return 0;

    QIcon appIcon;
    appIcon.addFile(QStringLiteral(":/icons/icon16.png"));
    appIcon.addFile(QStringLiteral(":/icons/icon32.png"));
    appIcon.addFile(QStringLiteral(":/icons/icon64.png"));
    appIcon.addFile(QStringLiteral(":/icons/icon128.png"));
    appIcon.addFile(QStringLiteral(":/icons/icon512.png"));
    app.setWindowIcon(appIcon);

    Backend backend;
    QQmlApplicationEngine engine;
    engine.setInitialProperties({
        {"injectedBackend", QVariant::fromValue(static_cast<QObject *>(&backend))},
        {"injectedLegalDocuments", legalDocuments()},
        {"injectedAppName", QStringLiteral(UTAUTTS_APP_NAME)},
        {"injectedRepositoryUrl", QUrl(QStringLiteral(UTAUTTS_APP_REPOSITORY))},
        {"injectedSelfTest", selfTest},
    });
    engine.loadFromModule("UtauTTS", "Main");
    if (engine.rootObjects().isEmpty()) {
        return -1;
    }

    backend.initialize();
    if (selfTest)
        return runSelfTest(backend, engine.rootObjects().constFirst());
    return app.exec();
}
