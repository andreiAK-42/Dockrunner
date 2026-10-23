#include <filesystem>
#include <iostream>
#include <fstream>
#include <string>
#include <vector>
#include <yaml-cpp/yaml.h>

namespace fs = std::filesystem;

namespace config {
    std::string filename = "config.yaml";

    bool isConfigExist() {
        if (fs::exists(filename)) {
            return true;
        } else {
            return false;
        }
    }

    YAML::Node loadConfig() {
        return YAML::LoadFile(filename);
    }

    bool createDefaultConfig() {
        YAML::Node config;

        config["agent_name"] = "Фобос";
        config["description"] = "Самый мощный сервер";

        config["central_server_url"] = "https://127.0.0.1:8089/api";
        config["agent_token"] = "токен";

        config["rabbit_mq_connection_string"] = "8089";

        config["sqlite_path"] = "qgent.db";
        config["workspace_dir"] = "./users_space/";

        config["heartbeat_interval_seconds"] = 30;
        config["port"] = 8089;
        
        std::ofstream fout(filename);
        fout << config;
        fout.close();

        return true;
    }
}