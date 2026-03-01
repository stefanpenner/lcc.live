import Foundation
import Observation

@Observable
class WeatherService {
    private var stations: [String: CachedStation] = [:]
    @ObservationIgnored private var fetchedSlugs: Set<String> = []
    @ObservationIgnored private let logger = Logger(category: .networking)

    private struct CachedStation {
        let station: WeatherStation
        let cachedAt: Date
    }

    private let ttl: TimeInterval = 30 * 60 // 30 minutes

    func station(for slug: String) -> WeatherStation? {
        guard let cached = stations[slug] else { return nil }
        if Date().timeIntervalSince(cached.cachedAt) > ttl {
            stations.removeValue(forKey: slug)
            fetchedSlugs.remove(slug)
            return nil
        }
        return cached.station
    }

    func cache(_ station: WeatherStation, for slug: String) {
        stations[slug] = CachedStation(station: station, cachedAt: Date())
        logger.debug("Cached weather for \(slug): \(station.AirTemperature ?? "nil")°F")
    }

    /// Fetch weather for all cameras that have a weatherStationId
    func fetchWeatherForCameras(_ items: [MediaItem], apiService: APIService) async {
        let camerasWithWeather = items.filter { $0.weatherStationId != nil && $0.slug != nil }
        logger.debug("Fetching weather for \(camerasWithWeather.count) cameras")

        for item in camerasWithWeather {
            guard let slug = item.slug else { continue }

            if fetchedSlugs.contains(slug), station(for: slug) != nil {
                continue
            }

            if let data = await apiService.fetchCameraDetail(slug: slug) {
                if let ws = data.WeatherStation {
                    await MainActor.run {
                        cache(ws, for: slug)
                        fetchedSlugs.insert(slug)
                    }
                }
            }
        }
    }
}
