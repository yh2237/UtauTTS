cmake_minimum_required(VERSION 3.20)
project(utautts_world_engine LANGUAGES CXX)

set(WORLD_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/world/src")

add_library(utautts-world-engine SHARED
    world_engine.cpp
    "${WORLD_ROOT}/cheaptrick.cpp"
    "${WORLD_ROOT}/common.cpp"
    "${WORLD_ROOT}/d4c.cpp"
    "${WORLD_ROOT}/fft.cpp"
    "${WORLD_ROOT}/harvest.cpp"
    "${WORLD_ROOT}/matlabfunctions.cpp"
    "${WORLD_ROOT}/synthesis.cpp"
)

target_include_directories(utautts-world-engine PRIVATE "${WORLD_ROOT}")
target_compile_features(utautts-world-engine PRIVATE cxx_std_17)
target_compile_definitions(utautts-world-engine PRIVATE UTAUTTS_WORLD_ENGINE_BUILD)
set_target_properties(utautts-world-engine PROPERTIES
    OUTPUT_NAME "utautts-world-engine"
    PREFIX ""
)

if(MSVC)
    target_compile_options(utautts-world-engine PRIVATE /W4 /EHsc)
else()
    target_compile_options(utautts-world-engine PRIVATE -Wall -Wextra -Wpedantic)
endif()

if(WIN32 AND CMAKE_CXX_COMPILER_ID STREQUAL "GNU")
    target_link_options(utautts-world-engine PRIVATE -static -static-libgcc -static-libstdc++)
endif()
