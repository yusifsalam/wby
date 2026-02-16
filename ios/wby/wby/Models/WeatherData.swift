import Foundation

struct WeatherResponse: Codable {
    let station: StationInfo
    let current: CurrentConditions
    let hourlyForecast: [HourlyForecast]
    let dailyForecast: [DailyForecast]

    enum CodingKeys: String, CodingKey {
        case station
        case current
        case hourlyForecast = "hourly_forecast"
        case dailyForecast = "daily_forecast"
    }

    init(station: StationInfo, current: CurrentConditions, hourlyForecast: [HourlyForecast], dailyForecast: [DailyForecast]) {
        self.station = station
        self.current = current
        self.hourlyForecast = hourlyForecast
        self.dailyForecast = dailyForecast
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        station = try c.decode(StationInfo.self, forKey: .station)
        current = try c.decode(CurrentConditions.self, forKey: .current)
        hourlyForecast = try c.decodeIfPresent([HourlyForecast].self, forKey: .hourlyForecast) ?? []
        dailyForecast = try c.decode([DailyForecast].self, forKey: .dailyForecast)
    }
}

struct StationInfo: Codable {
    let name: String
    let distanceKm: Double

    enum CodingKeys: String, CodingKey {
        case name
        case distanceKm = "distance_km"
    }
}

struct CurrentConditions: Codable {
    let temperature: Double?
    let feelsLike: Double?
    let windSpeed: Double?
    let windGust: Double?
    let windDirection: Double?
    let humidity: Double?
    let dewPoint: Double?
    let pressure: Double?
    let precipitation1h: Double?
    let precipitationIntensity: Double?
    let snowDepth: Double?
    let visibility: Double?
    let cloudCover: Double?
    let weatherCode: Double?
    let extra: [String: Double]?
    let observedAt: Date

    init(
        temperature: Double?,
        feelsLike: Double?,
        windSpeed: Double?,
        windGust: Double?,
        windDirection: Double?,
        humidity: Double?,
        dewPoint: Double? = nil,
        pressure: Double?,
        precipitation1h: Double? = nil,
        precipitationIntensity: Double? = nil,
        snowDepth: Double? = nil,
        visibility: Double? = nil,
        cloudCover: Double? = nil,
        weatherCode: Double? = nil,
        extra: [String: Double]? = nil,
        observedAt: Date
    ) {
        self.temperature = temperature
        self.feelsLike = feelsLike
        self.windSpeed = windSpeed
        self.windGust = windGust
        self.windDirection = windDirection
        self.humidity = humidity
        self.dewPoint = dewPoint
        self.pressure = pressure
        self.precipitation1h = precipitation1h
        self.precipitationIntensity = precipitationIntensity
        self.snowDepth = snowDepth
        self.visibility = visibility
        self.cloudCover = cloudCover
        self.weatherCode = weatherCode
        self.extra = extra
        self.observedAt = observedAt
    }

    enum CodingKeys: String, CodingKey {
        case temperature
        case feelsLike = "feels_like"
        case windSpeed = "wind_speed"
        case windGust = "wind_gust"
        case windDirection = "wind_direction"
        case humidity
        case dewPoint = "dew_point"
        case pressure
        case precipitation1h = "precipitation_1h"
        case precipitationIntensity = "precipitation_intensity"
        case snowDepth = "snow_depth"
        case visibility
        case cloudCover = "cloud_cover"
        case weatherCode = "weather_code"
        case extra
        case observedAt = "observed_at"
    }

    // FMI sometimes ships values only in `extra`. These accessors keep UI bound
    // to real observation values even when primary fields are nil.
    var resolvedTemperature: Double? { temperature ?? extraValue("t2m") }
    var resolvedWindSpeed: Double? { windSpeed ?? extraValue("ws_10min") }
    var resolvedWindGust: Double? { windGust ?? extraValue("wg_10min") }
    var resolvedWindDirection: Double? { windDirection ?? extraValue("wd_10min") }
    var resolvedHumidity: Double? { humidity ?? extraValue("rh") }
    var resolvedDewPoint: Double? { dewPoint ?? extraValue("td") }
    var resolvedPressure: Double? { pressure ?? extraValue("p_sea") }
    var resolvedPrecipitation1h: Double? { precipitation1h ?? extraValue("r_1h") }
    var resolvedPrecipitationIntensity: Double? { precipitationIntensity ?? extraValue("ri_10min") }
    var resolvedSnowDepth: Double? { snowDepth ?? extraValue("snow_aws") }
    var resolvedVisibility: Double? { visibility ?? extraValue("vis") }
    var resolvedCloudCover: Double? { cloudCover ?? extraValue("n_man") }
    var resolvedWeatherCode: Double? { weatherCode ?? extraValue("wawa") }
    var resolvedRadiationGlobal: Double? {
        guard let extra, !extra.isEmpty else { return nil }

        // Normalize keys once for case-insensitive matching.
        var normalized: [String: Double] = [:]
        normalized.reserveCapacity(extra.count)
        for (key, value) in extra {
            normalized[key.lowercased()] = value
        }

        let globalKeys = [
            "glob_1min",
            "globalradiation",
            "radiationglobal",
            "radiation_sw",
            "radiationsw",
            "swdn",
            "swdn_1min",
            "glob",
        ]
        for key in globalKeys {
            if let value = normalized[key] {
                return max(0, value)
            }
        }

        let direct = normalized["dir_1min"] ?? normalized["directradiation"]
        let diffuse = normalized["diff_1min"] ?? normalized["diffuseradiation"]
        if let direct, let diffuse {
            return max(0, direct + diffuse)
        }
        if let direct {
            return max(0, direct)
        }
        if let diffuse {
            return max(0, diffuse)
        }

        // Last-resort heuristic for future parameter variants.
        if let value = normalized.first(where: { $0.key.contains("glob") })?.value {
            return max(0, value)
        }
        return nil
    }

    var resolvedFeelsLike: Double? {
        if let feelsLike {
            return feelsLike
        }
        guard let temp = resolvedTemperature else { return nil }
        guard let windSpeed = resolvedWindSpeed else { return temp }
        let windKmh = windSpeed * 3.6
        if temp > 10 || windKmh < 4.8 {
            return temp
        }
        return 13.12 + 0.6215 * temp - 11.37 * pow(windKmh, 0.16) + 0.3965 * temp * pow(windKmh, 0.16)
    }

    private func extraValue(_ key: String) -> Double? {
        extra?[key]
    }
}

struct DailyForecast: Codable, Identifiable {
    let date: String
    let high: Double?
    let low: Double?
    let temperatureAvg: Double?
    let symbol: String?
    let windSpeedAvg: Double?
    let windDirectionAvg: Double?
    let humidityAvg: Double?
    let precipitationMm: Double?
    let precipitation1hSum: Double?
    let dewPointAvg: Double?
    let fogIntensityAvg: Double?
    let frostProbabilityAvg: Double?
    let severeFrostProbabilityAvg: Double?
    let geopHeightAvg: Double?
    let pressureAvg: Double?
    let highCloudCoverAvg: Double?
    let lowCloudCoverAvg: Double?
    let mediumCloudCoverAvg: Double?
    let middleAndLowCloudCoverAvg: Double?
    let totalCloudCoverAvg: Double?
    let hourlyMaximumGustMax: Double?
    let hourlyMaximumWindSpeedMax: Double?
    let popAvg: Double?
    let probabilityThunderstormAvg: Double?
    let potentialPrecipitationFormMode: Double?
    let potentialPrecipitationTypeMode: Double?
    let precipitationFormMode: Double?
    let precipitationTypeMode: Double?
    let radiationGlobalAvg: Double?
    let radiationLWAvg: Double?
    let weatherNumberMode: Double?
    let weatherSymbol3Mode: Double?
    let windUMSAvg: Double?
    let windVMSAvg: Double?
    let windVectorMSAvg: Double?
    let uvIndexAvg: Double?

    init(
        date: String,
        high: Double?,
        low: Double?,
        temperatureAvg: Double? = nil,
        symbol: String?,
        windSpeedAvg: Double?,
        windDirectionAvg: Double? = nil,
        humidityAvg: Double? = nil,
        precipitationMm: Double?,
        precipitation1hSum: Double? = nil,
        dewPointAvg: Double? = nil,
        fogIntensityAvg: Double? = nil,
        frostProbabilityAvg: Double? = nil,
        severeFrostProbabilityAvg: Double? = nil,
        geopHeightAvg: Double? = nil,
        pressureAvg: Double? = nil,
        highCloudCoverAvg: Double? = nil,
        lowCloudCoverAvg: Double? = nil,
        mediumCloudCoverAvg: Double? = nil,
        middleAndLowCloudCoverAvg: Double? = nil,
        totalCloudCoverAvg: Double? = nil,
        hourlyMaximumGustMax: Double? = nil,
        hourlyMaximumWindSpeedMax: Double? = nil,
        popAvg: Double? = nil,
        probabilityThunderstormAvg: Double? = nil,
        potentialPrecipitationFormMode: Double? = nil,
        potentialPrecipitationTypeMode: Double? = nil,
        precipitationFormMode: Double? = nil,
        precipitationTypeMode: Double? = nil,
        radiationGlobalAvg: Double? = nil,
        radiationLWAvg: Double? = nil,
        weatherNumberMode: Double? = nil,
        weatherSymbol3Mode: Double? = nil,
        windUMSAvg: Double? = nil,
        windVMSAvg: Double? = nil,
        windVectorMSAvg: Double? = nil,
        uvIndexAvg: Double? = nil
    ) {
        self.date = date
        self.high = high
        self.low = low
        self.temperatureAvg = temperatureAvg
        self.symbol = symbol
        self.windSpeedAvg = windSpeedAvg
        self.windDirectionAvg = windDirectionAvg
        self.humidityAvg = humidityAvg
        self.precipitationMm = precipitationMm
        self.precipitation1hSum = precipitation1hSum
        self.dewPointAvg = dewPointAvg
        self.fogIntensityAvg = fogIntensityAvg
        self.frostProbabilityAvg = frostProbabilityAvg
        self.severeFrostProbabilityAvg = severeFrostProbabilityAvg
        self.geopHeightAvg = geopHeightAvg
        self.pressureAvg = pressureAvg
        self.highCloudCoverAvg = highCloudCoverAvg
        self.lowCloudCoverAvg = lowCloudCoverAvg
        self.mediumCloudCoverAvg = mediumCloudCoverAvg
        self.middleAndLowCloudCoverAvg = middleAndLowCloudCoverAvg
        self.totalCloudCoverAvg = totalCloudCoverAvg
        self.hourlyMaximumGustMax = hourlyMaximumGustMax
        self.hourlyMaximumWindSpeedMax = hourlyMaximumWindSpeedMax
        self.popAvg = popAvg
        self.probabilityThunderstormAvg = probabilityThunderstormAvg
        self.potentialPrecipitationFormMode = potentialPrecipitationFormMode
        self.potentialPrecipitationTypeMode = potentialPrecipitationTypeMode
        self.precipitationFormMode = precipitationFormMode
        self.precipitationTypeMode = precipitationTypeMode
        self.radiationGlobalAvg = radiationGlobalAvg
        self.radiationLWAvg = radiationLWAvg
        self.weatherNumberMode = weatherNumberMode
        self.weatherSymbol3Mode = weatherSymbol3Mode
        self.windUMSAvg = windUMSAvg
        self.windVMSAvg = windVMSAvg
        self.windVectorMSAvg = windVectorMSAvg
        self.uvIndexAvg = uvIndexAvg
    }

    var id: String {
        date
    }

    var displayDate: Date? {
        let formatter = DateFormatter()
        formatter.dateFormat = "yyyy-MM-dd"
        return formatter.date(from: date)
    }

    enum CodingKeys: String, CodingKey {
        case date, high, low, symbol
        case temperatureAvg = "temperature_avg"
        case windSpeedAvg = "wind_speed_avg"
        case windDirectionAvg = "wind_direction_avg"
        case humidityAvg = "humidity_avg"
        case precipitationMm = "precipitation_mm"
        case precipitation1hSum = "precipitation_1h_sum"
        case dewPointAvg = "dew_point_avg"
        case fogIntensityAvg = "fog_intensity_avg"
        case frostProbabilityAvg = "frost_probability_avg"
        case severeFrostProbabilityAvg = "severe_frost_probability_avg"
        case geopHeightAvg = "geop_height_avg"
        case pressureAvg = "pressure_avg"
        case highCloudCoverAvg = "high_cloud_cover_avg"
        case lowCloudCoverAvg = "low_cloud_cover_avg"
        case mediumCloudCoverAvg = "medium_cloud_cover_avg"
        case middleAndLowCloudCoverAvg = "middle_and_low_cloud_cover_avg"
        case totalCloudCoverAvg = "total_cloud_cover_avg"
        case hourlyMaximumGustMax = "hourly_maximum_gust_max"
        case hourlyMaximumWindSpeedMax = "hourly_maximum_wind_speed_max"
        case popAvg = "pop_avg"
        case probabilityThunderstormAvg = "probability_thunderstorm_avg"
        case potentialPrecipitationFormMode = "potential_precipitation_form_mode"
        case potentialPrecipitationTypeMode = "potential_precipitation_type_mode"
        case precipitationFormMode = "precipitation_form_mode"
        case precipitationTypeMode = "precipitation_type_mode"
        case radiationGlobalAvg = "radiation_global_avg"
        case radiationLWAvg = "radiation_lw_avg"
        case weatherNumberMode = "weather_number_mode"
        case weatherSymbol3Mode = "weather_symbol3_mode"
        case windUMSAvg = "wind_ums_avg"
        case windVMSAvg = "wind_vms_avg"
        case windVectorMSAvg = "wind_vector_ms_avg"
        case uvIndexAvg = "uv_index_avg"
    }
}

struct HourlyForecast: Codable, Identifiable {
    let time: Date
    let temperature: Double?
    let windSpeed: Double?
    let windDirection: Double?
    let humidity: Double?
    let precipitation1h: Double?
    let uvCumulated: Double?
    let symbol: String?

    var id: Date { time }

    init(
        time: Date,
        temperature: Double?,
        windSpeed: Double? = nil,
        windDirection: Double? = nil,
        humidity: Double? = nil,
        precipitation1h: Double? = nil,
        uvCumulated: Double? = nil,
        symbol: String?
    ) {
        self.time = time
        self.temperature = temperature
        self.windSpeed = windSpeed
        self.windDirection = windDirection
        self.humidity = humidity
        self.precipitation1h = precipitation1h
        self.uvCumulated = uvCumulated
        self.symbol = symbol
    }

    enum CodingKeys: String, CodingKey {
        case time
        case temperature
        case windSpeed = "wind_speed"
        case windDirection = "wind_direction"
        case humidity
        case precipitation1h = "precipitation_1h"
        case uvCumulated = "uv_cumulated"
        case symbol
    }
}
