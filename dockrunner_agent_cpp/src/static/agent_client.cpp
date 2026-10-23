#include "agent_client.h"

drogon::HttpClientPtr geoHttp() {
    static drogon::HttpClientPtr client = drogon::HttpClient::newHttpClient("https://ipapi.co");

    return client;
}