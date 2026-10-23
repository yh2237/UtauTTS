cmake_minimum_required(VERSION 3.20)
project(utautts_openjtalk_engine LANGUAGES C CXX)

set(OPENJTALK_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/open_jtalk/src")
set(CMAKE_POSITION_INDEPENDENT_CODE ON)
set(BUILD_SHARED_LIBS OFF CACHE BOOL "" FORCE)
set(BUILD_PROGRAMS OFF CACHE BOOL "" FORCE)
add_subdirectory("${OPENJTALK_ROOT}" openjtalk EXCLUDE_FROM_ALL)

add_library(utautts-openjtalk-engine SHARED openjtalk_engine.cpp)
target_link_libraries(utautts-openjtalk-engine PRIVATE openjtalk)
foreach(directory mecab/src njd text2mecab mecab2njd njd_set_accent_phrase njd_set_accent_type
        njd_set_digit njd_set_long_vowel njd_set_pronunciation njd_set_unvoiced_vowel)
    target_include_directories(utautts-openjtalk-engine PRIVATE "${OPENJTALK_ROOT}/${directory}")
endforeach()
target_compile_features(utautts-openjtalk-engine PRIVATE cxx_std_17)
target_compile_definitions(utautts-openjtalk-engine PRIVATE
    HAVE_CONFIG_H DIC_VERSION=102 CHARSET_UTF_8 MECAB_UTF8_USE_ONLY MECAB_CHARSET=utf-8)
set_target_properties(utautts-openjtalk-engine PROPERTIES
    OUTPUT_NAME "utautts-openjtalk-engine"
    PREFIX ""
    WINDOWS_EXPORT_ALL_SYMBOLS ON
)

if(WIN32 AND CMAKE_CXX_COMPILER_ID STREQUAL "GNU")
    target_link_options(utautts-openjtalk-engine PRIVATE -static -static-libgcc -static-libstdc++ -s)
endif()
