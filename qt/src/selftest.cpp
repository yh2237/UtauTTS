#include "selftest.h"

#include "backend.h"

#include <QDir>
#include <QEventLoop>
#include <QDebug>
#include <QElapsedTimer>
#include <QFileInfo>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QJSValue>
#include <QCoreApplication>
#include <QMouseEvent>
#include <QMetaObject>
#include <QQuickItem>
#include <QQuickWindow>
#include <QSettings>
#include <QTemporaryDir>
#include <QTimer>
#include <QUrl>
#include <QVariantList>
#include <QVariantMap>
#include <tuple>
#include <utility>
#include <algorithm>

namespace {
constexpr int asyncTimeoutMS = 60000;

bool require(bool condition, const QString &message) {
    if (!condition)
        qCritical().noquote() << "self-test:" << message;
    return condition;
}

template<typename Signal, typename Trigger>
bool waitFor(Backend &backend, Signal signal, Trigger trigger, const QString &operation) {
    QEventLoop loop;
    QTimer timer;
    timer.setSingleShot(true);
    bool completed = false;
    QObject::connect(&backend, signal, &loop, [&] {
        completed = true;
        loop.quit();
    });
    QObject::connect(&backend, &Backend::errorChanged, &loop, [&] {
        if (!backend.error().isEmpty())
            loop.quit();
    });
    QObject::connect(&timer, &QTimer::timeout, &loop, &QEventLoop::quit);
    trigger();
    timer.start(asyncTimeoutMS);
    loop.exec();
    return require(backend.error().isEmpty(), operation + QStringLiteral(": ") + backend.error())
        && require(completed, operation + QStringLiteral(" timed out"));
}

void settle() {
    QEventLoop loop;
    QTimer::singleShot(300, &loop, &QEventLoop::quit);
    loop.exec();
}

bool captureInterface(QObject *rootObject, QQuickWindow *window, QQuickWindow *settings,
                      const QString &directory) {
    const QDir output(directory);
    if (!require(QDir().mkpath(directory), QStringLiteral("UI capture could not be initialized")))
        return false;
    window->show();
    settle();
    if (!require(window->grabWindow().save(output.filePath(QStringLiteral("normal.png"))),
                 QStringLiteral("UI capture failed")))
        return false;
    // 開いた状態の部品も撮る。メニューは利用者がクリックしたときと同じく、項目の triggered で開く。
    if (QObject *menuBar = rootObject->findChild<QObject *>(QStringLiteral("mainMenuBar"))) {
        QQuickItem *firstItem = nullptr;
        for (QQuickItem *item : menuBar->findChildren<QQuickItem *>()) {
            if (item->inherits("QQuickMenuBarItem")) {
                firstItem = item;
                break;
            }
        }
        if (firstItem) {
            QMetaObject::invokeMethod(firstItem, "triggered");
            settle();
            window->grabWindow().save(output.filePath(QStringLiteral("menu-open.png")));
            QMetaObject::invokeMethod(firstItem, "triggered");
            settle();
        }
    }
    if (QObject *combo = rootObject->findChild<QObject *>(QStringLiteral("voicebankCombo"))) {
        if (auto *popup = combo->property("popup").value<QObject *>()) {
            QMetaObject::invokeMethod(popup, "open");
            settle();
            window->grabWindow().save(output.filePath(QStringLiteral("combo-open.png")));
            QMetaObject::invokeMethod(popup, "close");
            settle();
        }
    }
    if (QObject *tabs = rootObject->findChild<QObject *>(QStringLiteral("pitchModeTabs"))) {
        tabs->setProperty("currentIndex", 1);
        settle();
        window->grabWindow().save(output.filePath(QStringLiteral("editor-extended.png")));
        tabs->setProperty("currentIndex", 0);
        settle();
    }
    if (QObject *dialogs = rootObject->findChild<QObject *>(QStringLiteral("mainDialogs"))) {
        for (const char *name : {"aboutDialog", "closeWarningDialog", "projectNameDialog"}) {
            auto *dialog = dialogs->property(name).value<QObject *>();
            if (!dialog)
                continue;
            QMetaObject::invokeMethod(dialog, "open");
            settle();
            window->grabWindow().save(output.filePath(QString::fromLatin1(name) + QStringLiteral(".png")));
            QMetaObject::invokeMethod(dialog, "close");
            settle();
        }
    }
    window->hide();
    settings->show();
    for (const auto &[page, name] : {
             std::pair{1, QStringLiteral("settings-synthesis.png")},
             std::pair{2, QStringLiteral("settings-shortcuts.png")},
             std::pair{3, QStringLiteral("settings-index.png")},
             std::pair{0, QStringLiteral("settings-behavior.png")}}) {
        settings->setProperty("currentPage", page);
        settle();
        if (!require(settings->grabWindow().save(output.filePath(name)),
                     QStringLiteral("settings UI capture failed")))
            return false;
    }
    if (QObject *table = settings->findChild<QObject *>(QStringLiteral("referenceIndexTable"))) {
        settings->setProperty("currentPage", 3);
        QMetaObject::invokeMethod(table, "sortBy", Q_ARG(QVariant, 3));
        settle();
        settings->grabWindow().save(output.filePath(QStringLiteral("settings-index-sorted.png")));
        table->setProperty("sortColumn", -1);
        QMetaObject::invokeMethod(table, "rebuildOrder");
        settings->setProperty("currentPage", 0);
    }
    settings->hide();
    // 補助の窓は遅延生成なので、開いてから探す。
    for (const auto &[opener, objectName, fileName] : {
             std::tuple{"openLicense", QStringLiteral("licenseWindow"), QStringLiteral("license.png")},
             std::tuple{"openDictionarySettings", QStringLiteral("dictionaryWindow"), QStringLiteral("dictionary.png")},
             std::tuple{"showVoicebankDetails", QStringLiteral("voicebankDetailsWindow"), QStringLiteral("voicebank-details.png")}}) {
        QMetaObject::invokeMethod(rootObject, opener);
        settle();
        if (auto *auxiliary = rootObject->findChild<QQuickWindow *>(objectName)) {
            auxiliary->grabWindow().save(output.filePath(fileName));
            if (QObject *table = auxiliary->findChild<QObject *>(QStringLiteral("dictionaryTable"))) {
                QMetaObject::invokeMethod(table, "editCell", Q_ARG(QVariant, 0), Q_ARG(QVariant, 1));
                settle();
                auxiliary->grabWindow().save(output.filePath(QStringLiteral("dictionary-editing.png")));
                QMetaObject::invokeMethod(table, "finishEditing");
            }
            auxiliary->hide();
        }
    }
    return true;
}

void captureSynthesized(QObject *rootObject, const QString &directory) {
    auto *window = qobject_cast<QQuickWindow *>(rootObject);
    if (!window)
        return;
    const QDir output(directory);
    window->show();
    settle();
    window->grabWindow().save(output.filePath(QStringLiteral("synthesized.png")));
    if (QObject *pitchEditor = rootObject->findChild<QObject *>(QStringLiteral("pitchEditor"))) {
        const QVariant moraWidth = pitchEditor->property("moraWidth");
        const int windowWidth = window->width();
        pitchEditor->setProperty("moraWidth", 192);
        window->setWidth(window->minimumWidth());
        settle();
        settle();
        window->grabWindow().save(output.filePath(QStringLiteral("synthesized-scrolled.png")));
        pitchEditor->setProperty("moraWidth", moraWidth);
        window->setWidth(windowWidth);
        settle();
    }
    if (QObject *tabs = rootObject->findChild<QObject *>(QStringLiteral("pitchModeTabs"))) {
        tabs->setProperty("currentIndex", 1);
        settle();
        window->grabWindow().save(output.filePath(QStringLiteral("synthesized-extended.png")));
        tabs->setProperty("currentIndex", 0);
        settle();
    }
    window->hide();
}

// UTAUTTS_PERF_UTTERANCES=N のとき、N 行のプロジェクトで主な操作の時間を測る。
double waitForFrame(QQuickWindow *window) {
    QElapsedTimer timer;
    timer.start();
    QEventLoop loop;
    const auto connection = QObject::connect(window, &QQuickWindow::frameSwapped, &loop, &QEventLoop::quit);
    QTimer::singleShot(2000, &loop, &QEventLoop::quit);
    window->requestUpdate();
    loop.exec();
    QObject::disconnect(connection);
    return timer.nsecsElapsed() / 1e6;
}

QString utteranceReading(QObject *model, int index) {
    QJSValue item;
    QMetaObject::invokeMethod(model, "get", Q_RETURN_ARG(QJSValue, item), Q_ARG(int, index));
    return item.property(QStringLiteral("reading")).toString();
}

void reportPerf(const char *name, const QList<double> &syncMs, const QList<double> &frameMs) {
    const auto stats = [](QList<double> values) {
        if (values.isEmpty())
            return QString();
        std::sort(values.begin(), values.end());
        double sum = 0;
        for (double value : values)
            sum += value;
        return QStringLiteral("avg %1 ms, median %2 ms, max %3 ms")
                .arg(sum / values.size(), 0, 'f', 2)
                .arg(values.at(values.size() / 2), 0, 'f', 2)
                .arg(values.last(), 0, 'f', 2);
    };
    qInfo().noquote() << "perf" << name << "| call:" << stats(syncMs) << "| until frame:" << stats(frameMs);
}

int runPerfBench(QObject *rootObject, int count) {
    auto *window = qobject_cast<QQuickWindow *>(rootObject);
    auto *model = rootObject->property("utterancesModel").value<QObject *>();
    if (!window || !model)
        return 1;
    window->show();
    waitForFrame(window);

    const QStringList sentences{
        QStringLiteral("こんにちは、今日はいい天気ですね。"),
        QStringLiteral("散歩に出かけるのはどうでしょう。"),
        QStringLiteral("明日の会議は午後三時から始まります。"),
        QStringLiteral("駅前の新しいパン屋さんに行ってみました。"),
        QStringLiteral("この資料を来週までにまとめておいてください。"),
        QStringLiteral("夜になると、星がとてもきれいに見えます。"),
        QStringLiteral("電車が遅れているので、少し遅刻しそうです。"),
        QStringLiteral("週末は家でゆっくり本を読むつもりです。"),
    };
    QVariantList utterances;
    for (int index = 0; index < count; ++index) {
        utterances.append(QVariantMap{
                {"text", QStringLiteral("%1番目。").arg(index + 1) + sentences.at(index % sentences.size())},
                {"language", "ja"}, {"voicebank_id", ""}, {"method", "waveform-synthesis"}});
    }
    const QVariantMap project{{"format", "utautts-project"}, {"format_version", 9},
                              {"utterances", utterances}, {"selected_index", 0}};

    QElapsedTimer timer;
    timer.start();
    QMetaObject::invokeMethod(rootObject, "applyLoadedProject",
                              Q_ARG(QVariant, QVariant(project)), Q_ARG(QVariant, QVariant()));
    const double loadCall = timer.nsecsElapsed() / 1e6;
    const double loadFrame = waitForFrame(window) + loadCall;
    int analyzed = 0;
    while (timer.elapsed() < 600000) {
        analyzed = 0;
        for (int index = 0; index < count; ++index)
            analyzed += utteranceReading(model, index).isEmpty() ? 0 : 1;
        if (analyzed == count)
            break;
        QEventLoop loop;
        QTimer::singleShot(50, &loop, &QEventLoop::quit);
        loop.exec();
    }
    qInfo().noquote() << "perf load" << count << "utterances | call:" << loadCall << "ms | first frame:"
                      << loadFrame << "ms | all analyzed:" << timer.elapsed() << "ms (" << analyzed << "/" << count << ")";
    // 解析のあとの抑揚の予測などが落ち着くのを待つ。
    {
        QEventLoop loop;
        QTimer::singleShot(3000, &loop, &QEventLoop::quit);
        loop.exec();
    }

    {
        int frames = 0;
        const auto counter = QObject::connect(window, &QQuickWindow::frameSwapped, window, [&frames] { ++frames; });
        QEventLoop loop;
        QTimer::singleShot(2000, &loop, &QEventLoop::quit);
        loop.exec();
        QObject::disconnect(counter);
        qInfo().noquote() << "perf idle frames in 2 s:" << frames;
    }
    QList<double> callMs;
    QList<double> frameMs;
    for (int step = 0; step < 20; ++step) {
        const int index = (step * 37) % count;
        timer.restart();
        QMetaObject::invokeMethod(rootObject, "selectUtterance", Q_ARG(QVariant, index), Q_ARG(QVariant, QVariant()));
        const double call = timer.nsecsElapsed() / 1e6;
        callMs.append(call);
        frameMs.append(call + waitForFrame(window));
    }
    reportPerf("select", callMs, frameMs);

    callMs.clear();
    frameMs.clear();
    const int typingIndex = count / 2;
    QString text = sentences.first();
    for (int step = 0; step < 30; ++step) {
        text += QChar(0x3042 + (step % 10) * 2);
        timer.restart();
        QMetaObject::invokeMethod(rootObject, "updateUtteranceText",
                                  Q_ARG(QVariant, typingIndex), Q_ARG(QVariant, text));
        const double call = timer.nsecsElapsed() / 1e6;
        callMs.append(call);
        frameMs.append(call + waitForFrame(window));
    }
    reportPerf("typing", callMs, frameMs);

    callMs.clear();
    frameMs.clear();
    if (QObject *list = rootObject->findChild<QObject *>(QStringLiteral("utteranceList"))) {
        const double height = list->property("contentHeight").toDouble();
        for (int step = 0; step < 30; ++step) {
            timer.restart();
            list->setProperty("contentY", height * step / 30.0);
            const double call = timer.nsecsElapsed() / 1e6;
            callMs.append(call);
            frameMs.append(call + waitForFrame(window));
        }
        reportPerf("scroll", callMs, frameMs);
    }

    QString longText;
    for (int index = 0; index < 8; ++index)
        longText += sentences.at(index);
    timer.restart();
    {
        QEventLoop loop;
        QObject::connect(rootObject->property("injectedBackend").value<QObject *>(), SIGNAL(analysisChanged()),
                         &loop, SLOT(quit()));
        QTimer::singleShot(10000, &loop, &QEventLoop::quit);
        QMetaObject::invokeMethod(rootObject, "updateUtteranceText", Q_ARG(QVariant, 0), Q_ARG(QVariant, longText));
        loop.exec();
    }
    qInfo().noquote() << "perf long text" << longText.size() << "chars, analyzed in" << timer.elapsed() << "ms";
    {
        QEventLoop loop;
        QTimer::singleShot(4000, &loop, &QEventLoop::quit);
        loop.exec();
    }
    callMs.clear();
    frameMs.clear();
    for (int step = 0; step < 10; ++step) {
        QMetaObject::invokeMethod(rootObject, "selectUtterance", Q_ARG(QVariant, 1), Q_ARG(QVariant, QVariant()));
        waitForFrame(window);
        timer.restart();
        QMetaObject::invokeMethod(rootObject, "selectUtterance", Q_ARG(QVariant, 0), Q_ARG(QVariant, QVariant()));
        const double call = timer.nsecsElapsed() / 1e6;
        callMs.append(call);
        frameMs.append(call + waitForFrame(window));
    }
    reportPerf("select long", callMs, frameMs);

    const auto hover = [&](const char *name, QQuickItem *item) {
        if (!item)
            return;
        QList<double> calls;
        QList<double> frames;
        const QPointF origin = item->mapToScene(QPointF(0, 0));
        for (int step = 0; step < 40; ++step) {
            const QPointF scene = origin + QPointF(item->width() * (step + 0.5) / 40.0, item->height() * 0.4);
            QMouseEvent event(QEvent::MouseMove, scene, scene, window->mapToGlobal(scene),
                              Qt::NoButton, Qt::NoButton, Qt::NoModifier);
            QElapsedTimer moveTimer;
            moveTimer.start();
            QCoreApplication::sendEvent(window, &event);
            const double call = moveTimer.nsecsElapsed() / 1e6;
            calls.append(call);
            frames.append(call + waitForFrame(window));
        }
        reportPerf(name, calls, frames);
    };
    hover("hover basic", rootObject->findChild<QQuickItem *>(QStringLiteral("pitchEditor")));
    if (QObject *tabs = rootObject->findChild<QObject *>(QStringLiteral("pitchModeTabs"))) {
        timer.restart();
        tabs->setProperty("currentIndex", 1);
        const double call = timer.nsecsElapsed() / 1e6;
        qInfo().noquote() << "perf open extended | call:" << call << "ms | until frame:" << call + waitForFrame(window) << "ms";
        QEventLoop loop;
        QTimer::singleShot(3000, &loop, &QEventLoop::quit);
        loop.exec();
        hover("hover extended", rootObject->findChild<QQuickItem *>(QStringLiteral("phonemeEditor")));
    }
    return 0;
}

QString firstID(const QVariantList &items) {
    return items.isEmpty() ? QString() : items.first().toMap().value(QStringLiteral("id")).toString();
}

QVariantMap sampleProject(const QString &voicebankID, const QString &modelID) {
    const QVariantMap utterance{
        {"text", QStringLiteral("こんにちは")}, {"voicebank_id", voicebankID},
        {"model_id", modelID}, {"alias_policy", "auto"},
        {"tone", "C4"}, {"color", ""}, {"mora_duration_ms", 120},
        {"pause_duration_ms", 180}, {"intonation", 1.0}, {"apply_pitch", true},
        {"pitch_points", QVariantList{}}, {"mora_durations_ms", QVariantList{}},
        {"mora_positions_ms", QVariantList{}}
    };
    return {{"format", "utautts-project"}, {"format_version", 5},
            {"utterances", QVariantList{utterance}}, {"selected_index", 0}};
}
}

int runSelfTest(Backend &backend, QObject *rootObject) {
    if (const int perfUtterances = qEnvironmentVariableIntValue("UTAUTTS_PERF_UTTERANCES"); perfUtterances > 0)
        return runPerfBench(rootObject, perfUtterances);
    const QString selfTestDirectory = qEnvironmentVariable("UTAUTTS_SELF_TEST_DIRECTORY");
    if (!require(backend.connected(), QStringLiteral("native backend is not connected"))
            || !require(QFileInfo::exists(QDir(selfTestDirectory).filePath(QStringLiteral("config.ini"))),
                        QStringLiteral("portable config file was not created"))
            || !require(!backend.voicebanks().isEmpty(), QStringLiteral("no bundled voicebank"))
            || !require(!backend.models().isEmpty(), QStringLiteral("no bundled prosody model"))
            || !require(!backend.settingDefinitions().isEmpty(), QStringLiteral("no synthesis settings")))
        return 1;

    QSettings migrationSettings(QDir(selfTestDirectory).filePath(QStringLiteral("config.ini")),
                                QSettings::IniFormat);
    if (!require(migrationSettings.value(QStringLiteral("migration/schema")).toInt() >= 1,
                 QStringLiteral("startup migration schema was not recorded"))
            || !require(!migrationSettings.contains(QStringLiteral("migration/pending_to")),
                        QStringLiteral("startup migration remained pending")))
        return 1;

    const QStringList languageCodes = backend.languageCodes();
    const QString resolvedLanguage = backend.resolvedLanguage();
    QJsonParseError languageError;
    const QJsonDocument languageDocument = QJsonDocument::fromJson(
            backend.loadLanguageFile(QStringLiteral("auto")).toUtf8(), &languageError);
    if (!require(!languageCodes.isEmpty() && languageCodes.first() == QStringLiteral("auto"),
                 QStringLiteral("automatic language option is unavailable"))
            || !require(languageCodes.contains(resolvedLanguage),
                        QStringLiteral("resolved UI language is unavailable: ") + resolvedLanguage)
            || !require(languageError.error == QJsonParseError::NoError && languageDocument.isObject(),
                        QStringLiteral("automatic language file could not be loaded")))
        return 1;

    if (!require(!backend.onboardingCompleted(),
                 QStringLiteral("onboarding should be pending on first run")))
        return 1;
    backend.setOnboardingCompleted(true);
    if (!require(backend.onboardingCompleted(),
                 QStringLiteral("onboarding completion was not recorded")))
        return 1;

    QVariant interfaceResult;
    if (!require(rootObject != nullptr
                 && QMetaObject::invokeMethod(rootObject, "runInterfaceSelfTest",
                                              Q_RETURN_ARG(QVariant, interfaceResult)),
                 QStringLiteral("QML interface self-test could not be invoked"))
            || !require(interfaceResult.toString().isEmpty(),
                        QStringLiteral("QML interface self-test: ") + interfaceResult.toString()))
        return 1;

    QTemporaryDir temporary;
    const QString captureDirectory = qEnvironmentVariable("UTAUTTS_UI_CAPTURE_DIR");
    if (!captureDirectory.isEmpty()) {
        auto *window = qobject_cast<QQuickWindow *>(rootObject);
        // 遅延生成の設定画面を、キャプチャ前に作る。
        if (!require(QMetaObject::invokeMethod(rootObject, "openSettings"),
                     QStringLiteral("settings UI could not be opened for capture")))
            return 1;
        auto *settings = rootObject->findChild<QQuickWindow *>(QStringLiteral("settingsWindow"));
        if (!require(window && settings, QStringLiteral("UI capture could not be initialized")))
            return 1;
        backend.setDictionaryEntries(QVariantList{
                QVariantMap{{"surface", "UtauTTS"}, {"reading", QStringLiteral("うたうてぃーてぃーえす")}},
                QVariantMap{{"surface", QStringLiteral("足立レイ")}, {"reading", QStringLiteral("あだちれい")}}});
        const bool previousDarkMode = backend.darkMode();
        for (const bool dark : {false, true}) {
            backend.setDarkMode(dark);
            if (!captureInterface(rootObject, window, settings,
                                  dark ? QDir(captureDirectory).filePath(QStringLiteral("dark")) : captureDirectory))
                return 1;
        }
        backend.setDarkMode(previousDarkMode);
        backend.setDictionaryEntries(QVariantList{});
    }
    if (!require(temporary.isValid(), QStringLiteral("temporary directory is unavailable")))
        return 1;

    // 実際のFileDialogから渡したURLが、ZIPを開く処理まで届くことを確認する。
    QObject *voicebankDialog = rootObject->findChild<QObject *>(QStringLiteral("voicebankAddDialog"));
    QFile invalidArchive(temporary.filePath(QStringLiteral("invalid-voicebank.zip")));
    if (!require(voicebankDialog && invalidArchive.open(QIODevice::WriteOnly),
                 QStringLiteral("voicebank dialog test could not be prepared")))
        return 1;
    invalidArchive.write("not a zip");
    invalidArchive.close();
    if (!require(voicebankDialog->setProperty("selectedFile", QUrl::fromLocalFile(invalidArchive.fileName()))
                 && QMetaObject::invokeMethod(voicebankDialog, "accepted")
                 && backend.busy(),
                 QStringLiteral("selected voicebank archive did not reach the backend: ") + backend.error()))
        return 1;
    {
        QEventLoop loop;
        QTimer timer;
        timer.setSingleShot(true);
        QObject::connect(&backend, &Backend::busyChanged, &loop, [&] {
            if (!backend.busy())
                loop.quit();
        });
        QObject::connect(&timer, &QTimer::timeout, &loop, &QEventLoop::quit);
        timer.start(asyncTimeoutMS);
        loop.exec();
    }
    // エラー文は表示言語に翻訳されるため、翻訳されないGo側の詳細で判定する。
    if (!require(!backend.busy() && backend.error().contains(QStringLiteral("not a valid zip file")),
                 QStringLiteral("voicebank archive error was not returned: ") + backend.error()))
        return 1;

    const QString voicebankID = firstID(backend.voicebanks());
    const QVariantMap model = backend.models().first().toMap();
    const QString modelID = model.value(QStringLiteral("id")).toString();
    if (!require(!voicebankID.isEmpty() && !modelID.isEmpty(),
                 QStringLiteral("metadata contains an empty id")))
        return 1;

    const QUrl projectURL = QUrl::fromLocalFile(temporary.filePath(QStringLiteral("smoke.utautts")));
    const QVariantMap project = sampleProject(voicebankID, modelID);
    if (!require(backend.saveProject(projectURL, project), backend.error()))
        return 1;
    const QVariantMap loadedProject = backend.loadProject(projectURL);
    if (!require(!loadedProject.contains(QStringLiteral("_error"))
                 && loadedProject.value(QStringLiteral("format")).toString() == QStringLiteral("utautts-project")
                 && loadedProject.value(QStringLiteral("utterances")).toList().size() == 1,
                 QStringLiteral("project round trip failed")))
        return 1;
    backend.clearRecentProjects();
    for (int timingSetting : {-1, 0, 1}) {
        QVariantMap compatibilityProject = project;
        QVariantMap utterance = project.value("utterances").toList().first().toMap();
        if (timingSetting >= 0)
            utterance.insert("speech_timing", timingSetting == 1);
        compatibilityProject.insert("utterances", QVariantList{utterance});
        QVariant savedState;
        if (!require(backend.saveProject(projectURL, compatibilityProject)
                     && QMetaObject::invokeMethod(rootObject, "loadProjectFrom",
                                                  Q_ARG(QVariant, QVariant(projectURL)))
                     && QMetaObject::invokeMethod(rootObject, "projectData",
                                                  Q_RETURN_ARG(QVariant, savedState)),
                     QStringLiteral("QML project migration could not be invoked")))
            return 1;
        const QVariantList rows = savedState.toMap().value("utterances").toList();
        if (!require(rootObject->property("projectFile").toUrl() == projectURL,
                     QStringLiteral("QML project load did not preserve the source URL")))
            return 1;
        if (!require(rows.size() == 1 && !rows.first().toMap().contains("speech_timing"),
                     QStringLiteral("legacy speech timing setting was not dropped")))
            return 1;
    }
    backend.rememberRecentProject(projectURL);
    if (!require(backend.recentProjects().size() == 1
                 && backend.recentProjects().first() == QFileInfo(projectURL.toLocalFile()).absoluteFilePath(),
                 QStringLiteral("recent project tracking failed")))
        return 1;
    backend.clearRecentProjects();

    const QUrl diagnosticsURL = QUrl::fromLocalFile(temporary.filePath(QStringLiteral("diagnostics.json")));
    const QVariantMap diagnosticContext{
        {"voicebank_id", voicebankID}, {"model_id", modelID},
        {"alias_policy", "auto"}, {"tone", "C4"}, {"mora_duration_ms", 120},
        {"pause_duration_ms", 180}, {"intonation_strength", 1.0}, {"apply_pitch", true},
        {"text", QStringLiteral("診断情報に含めない文章")},
    };
    if (!require(backend.exportDiagnosticReport(diagnosticsURL, diagnosticContext), backend.error()))
        return 1;
    QFile diagnosticsFile(diagnosticsURL.toLocalFile());
    if (!require(diagnosticsFile.open(QIODevice::ReadOnly), QStringLiteral("diagnostic report could not be read")))
        return 1;
    const QByteArray diagnosticsData = diagnosticsFile.readAll();
    const QJsonDocument diagnostics = QJsonDocument::fromJson(diagnosticsData);
    if (!require(diagnostics.isObject()
                 && diagnostics.object().value(QStringLiteral("format")).toString()
                    == QStringLiteral("utautts-diagnostic-report")
                 && !diagnosticsData.contains("診断情報に含めない文章"),
                 QStringLiteral("diagnostic report is invalid or contains input text")))
        return 1;

    if (!waitFor(backend, &Backend::analysisChanged,
                 [&] { backend.analyze(QStringLiteral("こんにちは"), QStringLiteral("self-test")); },
                 QStringLiteral("analysis")))
        return 1;
    const QJsonDocument analysis = QJsonDocument::fromJson(backend.analysisJson().toUtf8());
    if (!require(analysis.isObject() && !analysis.object().value(QStringLiteral("reading")).toString().isEmpty(),
                 QStringLiteral("analysis result is invalid")))
        return 1;

    const QVariantMap commonRequest{
        {"request_id", "self-test-prosody"}, {"text", QStringLiteral("こんにちは")},
        {"language", "ja"}, {"method", "waveform-synthesis"},
        {"voicebank_id", voicebankID}, {"model_id", modelID},
        {"alias_policy", "auto"}, {"tone", "C4"}, {"mora_duration_ms", 120},
        {"pause_duration_ms", 180}, {"intonation_strength", 1.0}, {"apply_pitch", true}
    };
    if (!waitFor(backend, &Backend::prosodyChanged,
                 [&] { backend.predictProsody(commonRequest); }, QStringLiteral("prosody prediction")))
        return 1;
    const QJsonDocument prosody = QJsonDocument::fromJson(backend.prosodyJson().toUtf8());
    if (!require(prosody.isObject()
                  && !prosody.object().value(QStringLiteral("mora_durations_ms")).toArray().isEmpty(),
                  QStringLiteral("prosody result is invalid")))
        return 1;

    // 失敗したrevisionを再試行できることを確認する。
    QVariant currentValue;
    if (!require(QMetaObject::invokeMethod(rootObject, "current", Q_RETURN_ARG(QVariant, currentValue)),
                 QStringLiteral("current utterance could not be inspected")))
        return 1;
    QVariantMap current = currentValue.metaType() == QMetaType::fromType<QJSValue>()
            ? currentValue.value<QJSValue>().toVariant().toMap() : currentValue.toMap();
    if (QObject *currentObject = currentValue.value<QObject *>()) {
        current.insert("utteranceId", currentObject->property("utteranceId"));
        current.insert("revision", currentObject->property("revision"));
    } else if (currentValue.metaType() == QMetaType::fromType<QJSValue>()) {
        const QJSValue object = currentValue.value<QJSValue>();
        current.insert("utteranceId", object.property("utteranceId").toVariant());
        current.insert("revision", object.property("revision").toVariant());
    }
    const int selected = rootObject->property("selectedIndex").toInt();
    if (!require(!current.value("utteranceId").toString().isEmpty(),
                 QStringLiteral("current utterance has no ID")))
        return 1;
    if (!waitFor(backend, &Backend::prosodyChanged, [&] {
            QMetaObject::invokeMethod(rootObject, "clearAutomaticProsody", Q_ARG(QVariant, QVariant(selected)));
            rootObject->setProperty("pendingProsodyUtteranceId", current.value("utteranceId"));
            rootObject->setProperty("pendingProsodyRevision", current.value("revision"));
            rootObject->setProperty("pendingProsodyInFlight", false);
            rootObject->setProperty("pendingProsodyPreviewIndex", selected);
            QMetaObject::invokeMethod(rootObject, "flushProsodyPreviewRequest");
        }, QStringLiteral("retrying prosody preview")))
        return 1;

    // 生成中の再生操作で二重合成しないことを確認する。
    const bool previousAutoPreview = backend.autoPreviewEnabled();
    backend.setAutoPreviewEnabled(true);
    if (!waitFor(backend, &Backend::previewReady, [&] {
            rootObject->setProperty("autoPreviewPending", true);
            QMetaObject::invokeMethod(rootObject, "refreshPreview");
            QMetaObject::invokeMethod(rootObject, "synthesizeCurrent");
            // テストでは実際の音を再生しない。
            rootObject->setProperty("autoplayPreview", false);
        }, QStringLiteral("UI preview synthesis")))
        return 1;
    int extraPreviews = 0;
    const auto previewConnection = QObject::connect(&backend, &Backend::previewReady,
                                                    [&] { ++extraPreviews; });
    QMetaObject::invokeMethod(rootObject, "refreshPreview");
    QEventLoop previewLoop;
    QTimer::singleShot(400, &previewLoop, &QEventLoop::quit);
    previewLoop.exec();
    QObject::disconnect(previewConnection);
    backend.setAutoPreviewEnabled(previousAutoPreview);
    if (!captureDirectory.isEmpty()) {
        const bool previousDarkMode = backend.darkMode();
        for (const bool dark : {false, true}) {
            backend.setDarkMode(dark);
            captureSynthesized(rootObject, dark ? QDir(captureDirectory).filePath(QStringLiteral("dark"))
                                                : captureDirectory);
        }
        backend.setDarkMode(previousDarkMode);
    }
    if (!require(extraPreviews == 0 && !rootObject->property("autoPreviewPending").toBool(),
                 QStringLiteral("current preview was generated twice")))
        return 1;

    {
        QVariantMap cancelRequest = commonRequest;
        cancelRequest.insert(QStringLiteral("text"), QStringLiteral("中止の確認です。"));
        QEventLoop loop;
        QTimer timer;
        timer.setSingleShot(true);
        bool finished = false;
        const auto finish = [&] {
            finished = true;
            loop.quit();
        };
        const auto canceled = QObject::connect(&backend, &Backend::synthesisCanceled, &loop, finish);
        const auto ready = QObject::connect(&backend, &Backend::previewReady, &loop, finish);
        QObject::connect(&timer, &QTimer::timeout, &loop, &QEventLoop::quit);
        backend.synthesize(cancelRequest);
        backend.cancelSynthesis();
        timer.start(asyncTimeoutMS);
        loop.exec();
        QObject::disconnect(canceled);
        QObject::disconnect(ready);
        if (!require(finished && !backend.busy() && backend.error().isEmpty(),
                     QStringLiteral("canceling synthesis did not finish cleanly: ") + backend.error()))
            return 1;
    }
    if (!waitFor(backend, &Backend::previewReady,
                 [&] { backend.synthesize(commonRequest); }, QStringLiteral("synthesis")))
        return 1;
    const QUrl firstPreviewURL = backend.previewUrl();
    if (!waitFor(backend, &Backend::previewReady,
                 [&] { backend.synthesize(commonRequest); }, QStringLiteral("cached synthesis"))
            || !require(backend.previewUrl() == firstPreviewURL,
                        QStringLiteral("cached synthesis created a different preview")))
        return 1;
    diagnosticsFile.close();
    if (!require(backend.exportDiagnosticReport(diagnosticsURL, diagnosticContext), backend.error()))
        return 1;
    if (!require(diagnosticsFile.open(QIODevice::ReadOnly), QStringLiteral("updated diagnostic report could not be read")))
        return 1;
    const QByteArray updatedDiagnosticsData = diagnosticsFile.readAll();
    if (!require(!updatedDiagnosticsData.contains("こんにちは")
                 && updatedDiagnosticsData.contains("<redacted>"),
                 QStringLiteral("diagnostic report did not redact synthesis text")))
        return 1;
    const QUrl wavURL = QUrl::fromLocalFile(temporary.filePath(QStringLiteral("smoke.wav")));
    const bool previousExportText = backend.exportTextWithWav();
    const bool previousExportLab = backend.exportLabWithWav();
    const QString previousExportEncoding = backend.exportTextEncoding();
    backend.setExportSettings(true, true, QStringLiteral("utf-8"));
    const bool previewSaved = backend.savePreview(wavURL);
    const QString previewError = backend.error();
    const bool wavValid = QFileInfo(wavURL.toLocalFile()).size() > 44;
    const bool textSaved = QFileInfo::exists(temporary.filePath(QStringLiteral("smoke.txt")));
    const bool labelSaved = QFileInfo::exists(temporary.filePath(QStringLiteral("smoke.lab")));
    QFile labelFile(temporary.filePath(QStringLiteral("smoke.lab")));
    const bool labelValid = labelFile.open(QIODevice::ReadOnly)
            && labelFile.readAll().contains(" ");
    backend.setExportSettings(previousExportText, previousExportLab, previousExportEncoding);
    if (!require(previewSaved, previewError)
            || !require(wavValid, QStringLiteral("saved WAV is empty"))
            || !require(textSaved, QStringLiteral("text sidecar was not saved"))
            || !require(labelSaved, QStringLiteral("label sidecar was not saved"))
            || !require(labelValid, QStringLiteral("label sidecar is invalid")))
        return 1;

    for (int flags = 0; flags < 4; ++flags) {
        for (const bool withSidecars : {false, true}) {
            const bool writeText = flags & 1;
            const bool writeLab = flags & 2;
            backend.setExportSettings(writeText, writeLab, QStringLiteral("utf-8"));
            backend.setExportTextEncoding(QStringLiteral("shift_jis"));
            const QString base = QStringLiteral("sidecars-%1-%2").arg(flags).arg(withSidecars);
            const bool saved = backend.savePreview(
                    QUrl::fromLocalFile(temporary.filePath(base + QStringLiteral(".wav"))), withSidecars);
            const QString error = backend.error();
            const bool settingsUnchanged = backend.exportTextWithWav() == writeText
                    && backend.exportLabWithWav() == writeLab
                    && backend.exportTextEncoding() == QStringLiteral("shift_jis");
            const bool expectedFiles = QFileInfo::exists(temporary.filePath(base + QStringLiteral(".txt")))
                            == (withSidecars || writeText)
                    && QFileInfo::exists(temporary.filePath(base + QStringLiteral(".lab")))
                            == (withSidecars || writeLab);
            backend.setExportSettings(previousExportText, previousExportLab, previousExportEncoding);
            if (!require(saved, error)
                    || !require(expectedFiles, QStringLiteral("sidecar export did not respect the request and settings"))
                    || !require(settingsUnchanged, QStringLiteral("sidecar export changed persistent settings")))
                return 1;
        }
    }

    const QUrl exoURL = backend.writeDragExo(QUrl::fromLocalFile(temporary.path()), QVariantList{wavURL}, 30);
    if (!require(exoURL.isLocalFile() && QFileInfo::exists(exoURL.toLocalFile()),
                 QStringLiteral("exo export failed: ") + backend.error()))
        return 1;

    const QUrl dictionaryURL = QUrl::fromLocalFile(temporary.filePath(QStringLiteral("dictionary.csv")));
    const QVariantList csvEntries{
            QVariantMap{{"surface", QStringLiteral("A,B")}, {"reading", QStringLiteral("えー\"びー")}},
            QVariantMap{{"surface", QStringLiteral("足立レイ")}, {"reading", QStringLiteral("あだちれい")}}};
    if (!require(backend.exportDictionary(dictionaryURL, csvEntries)
                 && backend.importDictionary(dictionaryURL).value(QStringLiteral("entries")).toList() == csvEntries,
                 QStringLiteral("dictionary CSV round trip failed")))
        return 1;
    backend.setDictionaryEntries(QVariantList{QVariantMap{{"surface", "UtauTTS"}, {"reading", "うたうてぃーてぃーえす"}}});
    if (!require(backend.dictionaryEntries().size() == 1 && !backend.dictionaryFingerprint().isEmpty(),
                 QStringLiteral("dictionary settings failed")))
        return 1;
    backend.setSynthesisDefaults(QStringLiteral("frame-intonation-tcn-v12"),
                                 QStringLiteral("D4"), QStringLiteral("cv-only"));
    backend.setSynthesisSetting(QStringLiteral("mora_duration_ms"), 130);
    backend.setSynthesisSetting(QStringLiteral("pause_duration_ms"), 190);
    backend.setSynthesisSetting(QStringLiteral("leading_preutterance_ms"), 45);
    backend.setSynthesisSetting(QStringLiteral("intonation_strength"), 2.5);
    backend.setSynthesisSetting(QStringLiteral("boundary_tone"), false);
    backend.setPreviewCacheFileCount(7);
    backend.setShortcutSequences("Ctrl+Enter", "Ctrl+S", "Ctrl+O", "Ctrl+D", "Delete", "Ctrl+Z", "Ctrl+Y");
    if (!require(backend.defaultMoraDuration() == 130 && backend.defaultPauseDuration() == 190
                 && backend.defaultLeadingPreutterance() == 45
                 && backend.defaultModelId() == QStringLiteral("frame-intonation-tcn-v12")
                 && backend.previewCacheFileCount() == 7
                 && backend.defaultIntonationStrength() == 2.5
                 && backend.defaultTone() == QStringLiteral("D4")
                 && backend.defaultAliasPolicy() == QStringLiteral("cv-only")
                 && backend.synthesisSetting(QStringLiteral("boundary_tone"), true).toBool() == false
                 && backend.undoShortcut() == QStringLiteral("Ctrl+Z"),
                 QStringLiteral("application settings failed")))
        return 1;

    const auto utteranceCount = [&] {
        auto *model = rootObject->property("utterancesModel").value<QObject *>();
        return model ? model->property("count").toInt() : -1;
    };
    rootObject->setProperty("projectDirty", false);
    QMetaObject::invokeMethod(rootObject, "requestNewProject");
    if (!require(utteranceCount() == 1 && rootObject->property("projectFile").toUrl().isEmpty()
                 && !rootObject->property("projectDirty").toBool(),
                 QStringLiteral("new project did not start empty")))
        return 1;
    rootObject->setProperty("projectDirty", true);
    QMetaObject::invokeMethod(rootObject, "requestNewProject");
    const bool waitedForConfirmation = rootObject->property("pendingNewProject").toBool();
    QMetaObject::invokeMethod(rootObject, "finishPendingProjectLoad", Q_ARG(QVariant, true));
    if (!require(waitedForConfirmation && utteranceCount() == 1 && !rootObject->property("projectDirty").toBool(),
                 QStringLiteral("new project did not ask before discarding changes")))
        return 1;

    qInfo() << "UtauTTS self-test passed";
    return 0;
}
