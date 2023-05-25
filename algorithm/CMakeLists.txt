cmake_minimum_required(VERSION 3.9)
project(algorithm C CXX)

option(WITH_DEBUG_SYMBOLS "With debug symbols" ON)
option(BUILD_TESTS "Build unit tests" ON)

if(WITH_DEBUG_SYMBOLS)
    set(DEBUG_SYMBOLS "-g")
endif()

include_directories(
    ${PROJECT_SOURCE_DIR}
    ${CMAKE_CURRENT_BINARY_DIR}
)

set(CMAKE_CXX_FLAGS "-O0 -pipe -Wall -W -fPIC -std=c++17 ${DEBUG_SYMBOLS} -Wno-sign-compare")
set(CMAKE_INSTALL_PREFIX ${CMAKE_BINARY_DIR})

include_directories(${CMAKE_SOURCE_DIR})

add_library(algorithm "")
target_sources(algorithm
    PRIVATE
    "graph/edge_weighted_digraph.cpp"
    "graph/edge_weighted_digraph.h"
    "graph/dijkstra.cpp"
    "graph/dijkstra.h"
)

add_executable(dijkstra_test "")
target_sources(dijkstra_test
    PRIVATE
    "test/dijkstra_test.cpp"
)
target_link_libraries(dijkstra_test algorithm)

install(TARGETS algorithm
    RUNTIME DESTINATION bin
    LIBRARY DESTINATION lib
    ARCHIVE DESTINATION lib 
)

install(TARGETS dijkstra_test
    RUNTIME DESTINATION test
)

install(FILES
    "${CMAKE_SOURCE_DIR}/graph/edge_weighted_digraph.h"
    "${CMAKE_SOURCE_DIR}/graph/dijkstra.h"
    DESTINATION "include/graph"
)

install(FILES
    "${CMAKE_SOURCE_DIR}/resources/tinyEWD.txt"
    DESTINATION "resources"
)

