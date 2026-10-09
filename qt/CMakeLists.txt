cmake_minimum_required(VERSION 3.21)

set(UTAUTTS_APPINFO_FILE "${CMAKE_CURRENT_LIST_DIR}/../internal/appinfo/appinfo.json")
file(READ "${UTAUTTS_APPINFO_FILE}" UTAUTTS_APPINFO_JSON)
string(JSON UTAUTTS_APP_NAME GET "${UTAUTTS_APPINFO_JSON}" name)
string(JSON UTAUTTS_APP_VERSION GET "${UTAUTTS_APPINFO_JSON}" version)
string(JSON UTAUTTS_UPDATE_SCHEMA GET "${UTAUTTS_APPINFO_JSON}" update_schema)
string(JSON UTAUTTS_MIGRATION_SCHEMA GET "${UTAUTTS_APPINFO_JSON}" migration_schema)
string(JSON UTAUTTS_APP_ORGANIZATION GET "${UTAUTTS_APPINFO_JSON}" organization)
string(JSON UTAUTTS_APP_REPOSITORY GET "${UTAUTTS_APPINFO_JSON}" repository)
# CMakeのVERSIONは数字だけなので、ベータ版の接尾辞（-beta.N）を外す。
string(REGEX REPLACE "^v([0-9]+\\.[0-9]+\\.[0-9]+).*$" "\\1" UTAUTTS_CMAKE_VERSION "${UTAUTTS_APP_VERSION}")
project(UtauTTSQt VERSION ${UTAUTTS_CMAKE_VERSION} LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)

if(EMSCRIPTEN)
    find_package(Qt6 6.5 REQUIRED COMPONENTS Quick QuickControls2 QuickControls2Fusion)
else()
    find_package(Qt6 6.5 REQUIRED COMPONENTS Quick QuickControls2 Multimedia Concurrent)
endif()
qt_standard_project_setup(REQUIRES 6.5)
qt_policy(SET QTP0004 NEW)

set(UTAUTTS_NATIVE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../build/native" CACHE PATH "Directory containing utautts_native shared library")

set(UTAUTTS_ICON_FILE "${CMAKE_CURRENT_SOURCE_DIR}/../icons/utautts.ico")
if(WIN32)
    if(NOT EXISTS "${UTAUTTS_ICON_FILE}")
        message(WARNING "${UTAUTTS_ICON_FILE} was not found. Add the Windows application icon to the icons directory.")
    endif()
    configure_file(
        "${CMAKE_CURRENT_SOURCE_DIR}/utautts.rc.in"
        "${CMAKE_CURRENT_BINARY_DIR}/utautts.rc"
        @ONLY
    )
endif()

qt_add_executable(utautts
    src/main.cpp
    src/backend.cpp
    src/backend.h
    src/selftest.cpp
    src/selftest.h
    src/utautts_abi.h
    src/platform/engine.h
    src/platform/media.h
    src/platform/platform.h
    src/platform/pen_cursor.h
)

if(EMSCRIPTEN)
    target_sources(utautts PRIVATE src/platform/engine_wasm.cpp src/platform/media_wasm.cpp)
else()
    target_sources(utautts PRIVATE src/platform/engine_native.cpp src/platform/media_native.cpp)
endif()
target_sources(utautts PRIVATE src/platform/media_register.cpp src/platform/platform.cpp
    src/platform/pen_cursor.cpp)

if(WIN32)
    target_sources(utautts PRIVATE "${CMAKE_CURRENT_BINARY_DIR}/utautts.rc")
    set_target_properties(utautts PROPERTIES WIN32_EXECUTABLE TRUE)
elseif(APPLE)
    # 同梱資源は.appの外に置き、resourceRoot()で解決する。
    set_target_properties(utautts PROPERTIES
        MACOSX_BUNDLE TRUE
        MACOSX_RPATH TRUE
        BUILD_RPATH "${UTAUTTS_NATIVE_DIR}"
        INSTALL_RPATH "@loader_path/../Frameworks"
    )
elseif(EMSCRIPTEN)
    # ホスト資産だけの変更でQt本体を再リンクしないよう、コピーを別ターゲットにする。
    add_custom_command(OUTPUT "${CMAKE_CURRENT_BINARY_DIR}/wasm-host.stamp"
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
            "${CMAKE_CURRENT_SOURCE_DIR}/wasm/index.html"
            "${CMAKE_CURRENT_SOURCE_DIR}/wasm/engine-loader.js"
            "${CMAKE_CURRENT_SOURCE_DIR}/wasm/engine-worker.js"
            "${CMAKE_CURRENT_SOURCE_DIR}/wasm/config.js"
            "${CMAKE_CURRENT_SOURCE_DIR}/wasm/asset-paths.js"
            "${CMAKE_CURRENT_SOURCE_DIR}/wasm/bootstrap.js"
            "$<TARGET_FILE_DIR:utautts>"
        COMMAND ${CMAKE_COMMAND} -E touch "${CMAKE_CURRENT_BINARY_DIR}/wasm-host.stamp"
        DEPENDS
            "${CMAKE_CURRENT_SOURCE_DIR}/wasm/index.html"
            "${CMAKE_CURRENT_SOURCE_DIR}/wasm/engine-loader.js"
            "${CMAKE_CURRENT_SOURCE_DIR}/wasm/engine-worker.js"
            "${CMAKE_CURRENT_SOURCE_DIR}/wasm/config.js"
            "${CMAKE_CURRENT_SOURCE_DIR}/wasm/asset-paths.js"
            "${CMAKE_CURRENT_SOURCE_DIR}/wasm/bootstrap.js"
        VERBATIM)
    add_custom_target(copy_wasm_host ALL DEPENDS "${CMAKE_CURRENT_BINARY_DIR}/wasm-host.stamp")
    add_dependencies(utautts copy_wasm_host)
else()
    set_target_properties(utautts PROPERTIES RUNTIME_OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/app")
    set_target_properties(utautts PROPERTIES BUILD_RPATH "$ORIGIN;${UTAUTTS_NATIVE_DIR}" INSTALL_RPATH "$ORIGIN")
endif()

set_source_files_properties(qml/UnitOverrides.js PROPERTIES QT_QML_SKIP_QMLDIR_ENTRY TRUE)
set_source_files_properties(qml/MoraPositions.js PROPERTIES QT_QML_SKIP_QMLDIR_ENTRY TRUE)
set_source_files_properties(qml/QmlCore.js PROPERTIES QT_QML_SKIP_QMLDIR_ENTRY TRUE)
set_source_files_properties(qml/UtteranceCore.js PROPERTIES QT_QML_SKIP_QMLDIR_ENTRY TRUE)
set_source_files_properties(qml/EditorValues.js PROPERTIES QT_QML_SKIP_QMLDIR_ENTRY TRUE)
set_source_files_properties(qml/HistoryCore.js PROPERTIES QT_QML_SKIP_QMLDIR_ENTRY TRUE)
set_source_files_properties(qml/IntonationLab.js PROPERTIES QT_QML_SKIP_QMLDIR_ENTRY TRUE)
set_source_files_properties(qml/SelfTest.js PROPERTIES QT_QML_SKIP_QMLDIR_ENTRY TRUE)
qt_add_qml_module(utautts
    URI UtauTTS
    VERSION 1.0
    QML_FILES
        qml/Main.qml
        qml/views/MainDialogs.qml
        qml/views/MainMenuBar.qml
        qml/views/MobileChrome.qml
        qml/AppCore.qml
        qml/UnitOverrides.js
        qml/MoraPositions.js
        qml/QmlCore.js
        qml/UtteranceCore.js
        qml/EditorValues.js
        qml/HistoryCore.js
        qml/IntonationLab.js
        qml/SelfTest.js
        qml/Translator.qml
        qml/shell/AdaptiveDialogWindow.qml
        qml/shell/SettingsWindow.qml
        qml/shell/DictionaryWindow.qml
        qml/shell/LicenseWindow.qml
        qml/shell/VoicebankDetailsWindow.qml
        qml/shell/OnboardingWindow.qml
        qml/shell/SynthesisLogWindow.qml
        qml/shell/DragSourceWindow.qml
        qml/views/EditorContent.qml
        qml/views/SynthesisLogContent.qml
        qml/views/LicenseContent.qml
        qml/views/OnboardingContent.qml
        qml/views/DictionaryContent.qml
        qml/views/VoicebankDetailsContent.qml
        qml/views/SettingsContent.qml
        qml/controls/WindowHeader.qml
        qml/controls/MobilePageDialog.qml
        qml/controls/SettingsResetButton.qml
        qml/controls/GrayscaleMenuItem.qml
        qml/controls/PlaybackControls.qml
        qml/controls/ParameterSlider.qml
        qml/controls/DragHud.qml
        qml/editors/PitchEditor.qml
        qml/editors/PhonemeEditor.qml
        qml/editors/IntonationEditorSurface.qml
        qml/editors/PitchHorizontalScrollBar.qml
)

set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon16.png"
    PROPERTIES QT_RESOURCE_ALIAS "icon16.png"
)
set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon32.png"
    PROPERTIES QT_RESOURCE_ALIAS "icon32.png"
)
set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon64.png"
    PROPERTIES QT_RESOURCE_ALIAS "icon64.png"
)
set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon128.png"
    PROPERTIES QT_RESOURCE_ALIAS "icon128.png"
)
set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon256.png"
    PROPERTIES QT_RESOURCE_ALIAS "icon256.png"
)
set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon512.png"
    PROPERTIES QT_RESOURCE_ALIAS "icon512.png"
)
set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/../tools/intonation-lab/japanese-v1.json"
    PROPERTIES QT_RESOURCE_ALIAS "japanese-v1.json"
)
qt_add_resources(utautts "intonationLabCorpus"
    PREFIX "/training"
    FILES "${CMAKE_CURRENT_SOURCE_DIR}/../tools/intonation-lab/japanese-v1.json"
)

qt_add_resources(utautts "appicon"
    PREFIX "/icons"
    FILES
        "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon16.png"
        "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon32.png"
        "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon64.png"
        "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon128.png"
        "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon256.png"
        "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon512.png"
)

if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    # PIEだけでは外部データ参照やCOPY再配置が残るため、Qtの要件に合わせる。
    target_compile_options(utautts PRIVATE -fPIC)
    target_link_options(utautts PRIVATE "LINKER:-z,nocopyreloc" "LINKER:-z,text")
endif()

# QMLとメタオブジェクトの生成コードがWindowsのCOFFセクション上限を超えるため。
if(MINGW)
    target_compile_options(utautts PRIVATE -Wa,-mbig-obj)
endif()

set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/../LICENSE"
    PROPERTIES QT_RESOURCE_ALIAS "LICENSE"
)
set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/../THIRD_PARTY_NOTICES.txt"
    PROPERTIES QT_RESOURCE_ALIAS "THIRD_PARTY_NOTICES.txt"
)
set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/../LICENSE-SCOPE.md"
    PROPERTIES QT_RESOURCE_ALIAS "LICENSE-SCOPE.md"
)
set(legal_files
    "${CMAKE_CURRENT_SOURCE_DIR}/../LICENSE"
    "${CMAKE_CURRENT_SOURCE_DIR}/../LICENSE-SCOPE.md"
    "${CMAKE_CURRENT_SOURCE_DIR}/../THIRD_PARTY_NOTICES.txt"
)
if(WIN32)
    set_source_files_properties(
        "${CMAKE_CURRENT_SOURCE_DIR}/../THIRD_PARTY_NOTICES-WINDOWS-GUI.txt"
        PROPERTIES QT_RESOURCE_ALIAS "THIRD_PARTY_NOTICES-WINDOWS-GUI.txt"
    )
    list(APPEND legal_files
        "${CMAKE_CURRENT_SOURCE_DIR}/../THIRD_PARTY_NOTICES-WINDOWS-GUI.txt"
    )
endif()
if(APPLE)
    set_source_files_properties(
        "${CMAKE_CURRENT_SOURCE_DIR}/../THIRD_PARTY_NOTICES-MACOS-GUI.txt"
        PROPERTIES QT_RESOURCE_ALIAS "THIRD_PARTY_NOTICES-MACOS-GUI.txt"
    )
    list(APPEND legal_files
        "${CMAKE_CURRENT_SOURCE_DIR}/../THIRD_PARTY_NOTICES-MACOS-GUI.txt"
    )
endif()
qt_add_resources(utautts "legal"
    PREFIX "/legal"
    FILES ${legal_files}
)

file(GLOB lang_files CONFIGURE_DEPENDS
    "${CMAKE_CURRENT_SOURCE_DIR}/lang/*.json"
)
foreach(lang_file ${lang_files})
    get_filename_component(lang_name "${lang_file}" NAME)
    set_source_files_properties("${lang_file}" PROPERTIES QT_RESOURCE_ALIAS "${lang_name}")
endforeach()
qt_add_resources(utautts "lang"
    PREFIX "/lang"
    FILES
        ${lang_files}
)

# licenses/MATERIAL-SYMBOLS.txtの帰属表示に従うこと。
file(GLOB font_files CONFIGURE_DEPENDS
    "${CMAKE_CURRENT_SOURCE_DIR}/fonts/*.ttf"
)
foreach(font_file ${font_files})
    get_filename_component(font_name "${font_file}" NAME)
    set_source_files_properties("${font_file}" PROPERTIES QT_RESOURCE_ALIAS "${font_name}")
endforeach()
qt_add_resources(utautts "fonts"
    PREFIX "/fonts"
    FILES
        ${font_files}
)

if(NOT EMSCRIPTEN)
    if(WIN32)
        target_link_libraries(utautts PRIVATE "${UTAUTTS_NATIVE_DIR}/utautts_native.dll.a")
    elseif(APPLE)
        target_link_libraries(utautts PRIVATE "${UTAUTTS_NATIVE_DIR}/libutautts_native.dylib")
    else()
        target_link_libraries(utautts PRIVATE "${UTAUTTS_NATIVE_DIR}/libutautts_native.so")
    endif()
endif()

target_include_directories(utautts PRIVATE src)
target_compile_definitions(utautts PRIVATE
    UTAUTTS_APP_NAME="${UTAUTTS_APP_NAME}"
    UTAUTTS_VERSION="${UTAUTTS_APP_VERSION}"
    UTAUTTS_UPDATE_SCHEMA="${UTAUTTS_UPDATE_SCHEMA}"
    UTAUTTS_MIGRATION_SCHEMA="${UTAUTTS_MIGRATION_SCHEMA}"
    UTAUTTS_APP_ORGANIZATION="${UTAUTTS_APP_ORGANIZATION}"
    UTAUTTS_APP_REPOSITORY="${UTAUTTS_APP_REPOSITORY}"
)
target_link_libraries(utautts PRIVATE Qt6::Quick Qt6::QuickControls2)
if(EMSCRIPTEN)
    target_link_libraries(utautts PRIVATE Qt6::QuickControls2Fusion)
    target_compile_definitions(utautts PRIVATE UTAUTTS_WASM)
else()
    target_link_libraries(utautts PRIVATE Qt6::Multimedia Qt6::Concurrent)
endif()

install(TARGETS utautts RUNTIME DESTINATION . BUNDLE DESTINATION .)
