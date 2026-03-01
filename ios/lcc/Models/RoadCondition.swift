import Foundation

struct RoadCondition: Codable, Identifiable, Hashable {
    let Id: Int
    let SourceId: String
    let RoadCondition: String
    let WeatherCondition: String
    let Restriction: String
    let RoadwayName: String
    let LastUpdated: Int64

    var id: Int { Id }

    var lastUpdatedDate: Date {
        Date(timeIntervalSince1970: TimeInterval(LastUpdated))
    }

    var timeAgo: String {
        let seconds = Int(Date().timeIntervalSince(lastUpdatedDate))
        if seconds < 60 { return "\(seconds)s ago" }
        let minutes = seconds / 60
        if minutes < 60 { return "\(minutes)m ago" }
        let hours = minutes / 60
        return "\(hours)h ago"
    }

    var hasRestriction: Bool {
        !Restriction.isEmpty
            && Restriction.lowercased() != "no restrictions"
            && Restriction.lowercased() != "none"
    }
}

struct UDOTResponse: Codable {
    let roadConditions: [RoadCondition]
    let lastUpdated: Int64
}
