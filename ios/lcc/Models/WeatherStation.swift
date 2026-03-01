import Foundation

struct WeatherStation: Codable, Hashable {
    let Id: Int
    let StationName: String
    let AirTemperature: String?
    let SurfaceTemp: String?
    let SubSurfaceTemp: String?
    let SurfaceStatus: String?
    let RelativeHumidity: String?
    let DewpointTemp: String?
    let Precipitation: String?
    let WindSpeedAvg: String?
    let WindSpeedGust: String?
    let WindDirection: String?
    let LastUpdated: Int64

    var airTempF: Int? {
        guard let str = AirTemperature, let val = Double(str) else { return nil }
        return Int(val.rounded())
    }
}

struct CameraPageData: Codable {
    let Camera: CameraJSON
    let CanyonName: String
    let CanyonPath: String
    let ImageURL: String
    let WeatherStation: WeatherStation?
}

struct CameraJSON: Codable {
    let id: String
    let kind: String
    let src: String
    let alt: String
    let canyon: String
    let weatherStationId: Int?
}
