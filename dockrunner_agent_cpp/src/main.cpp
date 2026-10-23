#include <iostream>
#include "internal/geo/geo.h"
#include "config/config.h"
#include "internal/database/database.h"
#include <drogon/drogon.h>
#include <spdlog/spdlog.h> 
#include <spdlog/sinks/daily_file_sink.h>

#ifdef _WIN32
    #include <windows.h>
#endif


std::shared_ptr<spdlog::logger> initLogging() {
    auto logger = spdlog::daily_logger_mt("base_logger", "logs/agent.txt", 0, 0);

    spdlog::flush_every(std::chrono::seconds(3));

    return logger;
}

int main() {
#ifdef _WIN32
    SetConsoleOutputCP(CP_UTF8);
    SetConsoleCP(CP_UTF8);
#endif

    auto log = initLogging();

    log->info("Запуск приложения. Ожидание загрузки конфигурации...");

    if (config::isConfigExist()) {
        log->info("Конфигурация найдена. Загрузка...");
    }
    else {
        log->warn("Конфигурация не найдена. Загрузка базовой конфигурации для заполнения и остановка программы.");
        config::createDefaultConfig();

        return 0;
    }

    auto config = config::loadConfig();
    
    log->warn("Конфигурация загружена. Проверка базы данных");
 
    if (database::initialization(config["sqlite_path"].as<std::string>().c_str())) {
        log->info("База данных успешно инициализирована");
    }
    else {
        log->warn("Не удалось загрузить базу данных, завершение программы.");
        return 1;
    }

    drogon::app().addListener("0.0.0.0", config["port"].as<int>()).run();
    
    return 0;
}