#include <iostream>
#include <sqlite3.h>
#include <spdlog/spdlog.h>

namespace database {
    
bool initialization(const char *database_path) {
    auto logger = spdlog::get("base_logger");

    sqlite3 *db = nullptr;
    int returnCode = sqlite3_open(database_path, &db);

    if (returnCode != SQLITE_OK) {
        logger->error("needInit(): Не удалось открыть базу данных: {}", sqlite3_errmsg(db));
        return 1;
    }

    const char *sql = R"sql(
        CREATE TABLE IF NOT EXISTS deployments (
	    id TEXT PRIMARY KEY,
	    app_id TEXT NOT NULL,
	    app_name TEXT NOT NULL,
	    owner TEXT NOT NULL DEFAULT '',
	    repo_url TEXT NOT NULL DEFAULT '',
	    name TEXT NOT NULL DEFAULT '',
	    image TEXT NOT NULL,
	    container TEXT NOT NULL DEFAULT '',
	    status TEXT NOT NULL DEFAULT 'created',
	    ports TEXT NOT NULL DEFAULT '[]',
	    volumes TEXT NOT NULL DEFAULT '[]',
	    env TEXT NOT NULL DEFAULT '{}',
	    created_at TEXT NOT NULL,
	    updated_at TEXT NOT NULL);
    )sql";

    char *err = nullptr;
    returnCode = sqlite3_exec(db, sql, nullptr, nullptr, &err);

    if (returnCode != SQLITE_OK) {
        logger->error("SQL error: {}", err);
        sqlite3_free(err);
    }

    sqlite3_close(db);
    return true;
}
}