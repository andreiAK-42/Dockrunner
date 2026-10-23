#pragma once
#include <yaml-cpp/yaml.h>

namespace config
{
    bool isConfigExist();
    YAML::Node loadConfig();
    bool createDefaultConfig();
}