#include "platform/media.h"
#include "platform/platform.h"

#include <QApplication>
#include <QQmlComponent>
#include <QQmlContext>
#include <QQmlEngine>
#include <QQuickStyle>
#include <QtQuickTest/quicktest.h>
#include <QtQml/qqmlextensionplugin.h>

Q_IMPORT_QML_PLUGIN(UtauTTSStylePlugin)

class QuickTestSetup : public QObject {
    Q_OBJECT
public slots:
    void qmlEngineAvailable(QQmlEngine *engine) {
        QQmlComponent imports(engine);
        imports.setData("import QtQuick\nimport UtauTTS 1.0\nQtObject {}", QUrl());
        if (imports.isError())
            qFatal("%s", qPrintable(imports.errorString()));
        auto *theme = engine->singletonInstance<QObject *>(qmlTypeId("UtauTTS", 1, 0, "Theme"));
        engine->rootContext()->setContextProperty("Theme", theme);
    }
};

int main(int argc, char **argv) {
    QApplication app(argc, argv);
    QQuickStyle::setStyle("UtauTTSStyle");
    registerPlatformSingleton();
    registerMediaTypes();
    QuickTestSetup setup;
    return quick_test_main_with_setup(argc, argv, "utautts", QUICK_TEST_SOURCE_DIR, &setup);
}

#include "quicktest.moc"
