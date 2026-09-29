# find HTSEngine
find_path(HTS_ENGINE_INCLUDE_DIR HTS_engine.h)
find_library(HTS_ENGINE_LIB hts_engine_API)
if(NOT HTS_ENGINE_LIB)
  # fallback
  find_library(HTS_ENGINE_LIB HTSEngine)
endif()

if(NOT HTS_ENGINE_INCLUDE_DIR OR NOT HTS_ENGINE_LIB)
  message(FATAL_ERROR "Required HTSEngine not found")
endif()

add_executable(open_jtalk open_jtalk.c)
set_target_properties(open_jtalk PROPERTIES LINKER_LANGUAGE CXX)
target_link_libraries(open_jtalk PRIVATE openjtalk "${HTS_ENGINE_LIB}")
target_include_directories(open_jtalk PRIVATE "${HTS_ENGINE_INCLUDE_DIR}")
install(TARGETS open_jtalk DESTINATION bin)
