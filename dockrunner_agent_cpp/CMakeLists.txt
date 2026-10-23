cmake_minimum_required(VERSION 3.16)
project(dockrunner_agent_cpp LANGUAGES CXX)

set(CMAKE_EXPORT_COMPILE_COMMANDS ON)
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

if(MSVC)
    add_compile_options(/W4 /permissive-)
else()
    add_compile_options(-Wall -Wextra -Wpedantic)
endif()

find_package(Drogon CONFIG REQUIRED)
find_package(spdlog CONFIG REQUIRED)
find_package(Threads REQUIRED)
find_package(yaml-cpp REQUIRED)
find_package(unofficial-sqlite3 CONFIG REQUIRED)

file(GLOB_RECURSE SOURCES CONFIGURE_DEPENDS src/*.cpp)
add_executable(DockrunnerAgent ${SOURCES}
    src/config/config.h
    src/config/config.cpp
    src/internal/api/api.cpp
    src/internal/database/database.cpp
    src/internal/database/database.h)

target_include_directories(DockrunnerAgent PRIVATE src)

target_link_libraries(DockrunnerAgent PRIVATE
    Drogon::Drogon
    spdlog::spdlog
    Threads::Threads
    yaml-cpp::yaml-cpp
    unofficial::sqlite3::sqlite3
)