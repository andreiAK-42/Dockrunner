#include "geo.h"
#include <iostream>
#include "static/agent_client.h"

#include <drogon/drogon.h>
#include <spdlog/spdlog.h>

namespace geo {

struct geoResponse {
    std::string ip;
    std::string city;
    double latitude;
    double longitude;
};

void fetchGeo(std::function<void(std::optional<geoResponse>)> callback) {
    auto logger = spdlog::get("base_logger");
    auto req = drogon::HttpRequest::newHttpRequest();

    req->setMethod(drogon::Get);
    req->setPath("/json/");

    geoHttp()->sendRequest(req,
        [logger, callback = std::move(callback)](drogon::ReqResult r, const drogon::HttpResponsePtr& resp) {
            if (r != drogon::ReqResult::Ok) {
                logger->error("fetchGeo(): запрос не удался: {}", drogon::to_string_view(r));
                callback(std::nullopt);
            }

            auto json = resp->getJsonObject();
            
            if (!json) {
                logger->warn("fetchGeo(): неожиданный ответ: {}", std::string(resp->getBody()));
                callback(std::nullopt);
            }

            geoResponse geoResponse;
            geoResponse.ip = (*json)["ip"].asString();
            geoResponse.city = (*json)["city"].asString();
            geoResponse.latitude = (*json)["latitude"].asDouble();
            geoResponse.longitude = (*json)["longitude"].asDouble();

            callback(geoResponse);
        });
    }
}