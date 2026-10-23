cmake_minimum_required(VERSION 3.20)
project(utautts_native LANGUAGES C CXX)

# Open JTalkのCMakeLists.txtは古いcmake_minimum_requiredを書いている。
set(CMAKE_POLICY_VERSION_MINIMUM 3.5)
add_subdirectory(world-engine)
add_subdirectory(openjtalk-engine)
