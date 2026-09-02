cmake_minimum_required(VERSION 3.21)

set(UTAUTTS_APPINFO_FILE "${CMAKE_CURRENT_LIST_DIR}/../internal/appinfo/appinfo.json")
file(READ "${UTAUTTS_APPINFO_FILE}" UTAUTTS_APPINFO_JSON)
string(JSON UTAUTTS_APP_NAME GET "${UTAUTTS_APPINFO_JSON}" name)
string(JSON UTAUTTS_APP_VERSION GET "${UTAUTTS_APPINFO_JSON}" version)
string(JSON UTAUTTS_APP_ORGANIZATION GET "${UTAUTTS_APPINFO_JSON}" organization)
string(JSON UTAUTTS_APP_REPOSITORY GET "${UTAUTTS_APPINFO_JSON}" repository)
string(REGEX REPLACE "^v" "" UTAUTTS_CMAKE_VERSION "${UTAUTTS_APP_VERSION}")
project(UtauTTSQt VERSION ${UTAUTTS_CMAKE_VERSION} LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)

find_package(Qt6 6.5 REQUIRED COMPONENTS Quick QuickControls2 Multimedia Concurrent)
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
)

if(WIN32)
    target_sources(utautts PRIVATE "${CMAKE_CURRENT_BINARY_DIR}/utautts.rc")
    set_target_properties(utautts PROPERTIES WIN32_EXECUTABLE TRUE)
else()
    set_target_properties(utautts PROPERTIES RUNTIME_OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/app")
    set_target_properties(utautts PROPERTIES BUILD_RPATH "$ORIGIN;${UTAUTTS_NATIVE_DIR}" INSTALL_RPATH "$ORIGIN")
endif()

qt_add_qml_module(utautts
    URI UtauTTS
    VERSION 1.0
    QML_FILES
        qml/Main.qml
        qml/EditorContent.qml
        qml/PitchEditor.qml
        qml/PitchHorizontalScrollBar.qml
        qml/PlaybackControls.qml
        qml/DragSourceWindow.qml
        qml/SynthesisLogWindow.qml
        qml/SettingsWindow.qml
        qml/SettingsResetButton.qml
        qml/BreezeIcon.qml
        qml/GrayscaleMenuItem.qml
        qml/DictionaryWindow.qml
        qml/LicenseWindow.qml
        qml/UsageWindow.qml
        qml/VoicebankDetailsWindow.qml
        qml/ProsodyTrainingWindow.qml
        qml/Translator.qml
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
    "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon512.png"
    PROPERTIES QT_RESOURCE_ALIAS "icon512.png"
)
qt_add_resources(utautts "appicon"
    PREFIX "/icons"
    FILES
        "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon16.png"
        "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon32.png"
        "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon64.png"
        "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon128.png"
        "${CMAKE_CURRENT_SOURCE_DIR}/../icons/icon512.png"
)

set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/assets/icons/breeze-view-refresh.svg"
    PROPERTIES QT_RESOURCE_ALIAS "breeze-view-refresh.svg"
)
set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/assets/icons/breeze-edit-delete.svg"
    PROPERTIES QT_RESOURCE_ALIAS "breeze-edit-delete.svg"
)
set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/assets/icons/breeze-overflow-menu.svg"
    PROPERTIES QT_RESOURCE_ALIAS "breeze-overflow-menu.svg"
)
qt_add_resources(utautts "uiicons"
    PREFIX "/icons"
    FILES
        "${CMAKE_CURRENT_SOURCE_DIR}/assets/icons/breeze-view-refresh.svg"
        "${CMAKE_CURRENT_SOURCE_DIR}/assets/icons/breeze-edit-delete.svg"
        "${CMAKE_CURRENT_SOURCE_DIR}/assets/icons/breeze-overflow-menu.svg"
)

set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/../LICENSE"
    PROPERTIES QT_RESOURCE_ALIAS "LICENSE"
)
set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/../THIRD_PARTY_NOTICES.txt"
    PROPERTIES QT_RESOURCE_ALIAS "THIRD_PARTY_NOTICES.txt"
)
set(legal_files
    "${CMAKE_CURRENT_SOURCE_DIR}/../LICENSE"
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
qt_add_resources(utautts "legal"
    PREFIX "/legal"
    FILES ${legal_files}
)

# UI language files (JSON). Translator.qml loads qrc:/lang/<code>.json.
# Adding a new <code>.json here automatically makes it a selectable language.
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

set_source_files_properties(
    "${CMAKE_CURRENT_SOURCE_DIR}/prosody-prompts-ja-v1.json"
    PROPERTIES QT_RESOURCE_ALIAS "prosody-prompts-ja-v1.json"
)
qt_add_resources(utautts "data"
    PREFIX "/data"
    FILES
        "${CMAKE_CURRENT_SOURCE_DIR}/prosody-prompts-ja-v1.json"
)

if(WIN32)
    target_link_libraries(utautts PRIVATE "${UTAUTTS_NATIVE_DIR}/utautts_native.dll.a")
else()
    target_link_libraries(utautts PRIVATE "${UTAUTTS_NATIVE_DIR}/libutautts_native.so")
endif()

target_include_directories(utautts PRIVATE src)
target_compile_definitions(utautts PRIVATE
    UTAUTTS_APP_NAME="${UTAUTTS_APP_NAME}"
    UTAUTTS_VERSION="${UTAUTTS_APP_VERSION}"
    UTAUTTS_APP_ORGANIZATION="${UTAUTTS_APP_ORGANIZATION}"
    UTAUTTS_APP_REPOSITORY="${UTAUTTS_APP_REPOSITORY}"
)
target_link_libraries(utautts PRIVATE Qt6::Quick Qt6::QuickControls2 Qt6::Multimedia Qt6::Concurrent)

install(TARGETS utautts RUNTIME DESTINATION . BUNDLE DESTINATION .)
