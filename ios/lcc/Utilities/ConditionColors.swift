import SwiftUI

enum ConditionColors {
    static func roadConditionColor(_ condition: String) -> Color {
        switch condition.lowercased() {
        case "dry": return .green
        case "wet": return .blue
        case "snowy", "snow covered", "snow": return Color(red: 0.53, green: 0.81, blue: 0.98)
        case "icy", "ice": return .indigo
        default: return .secondary
        }
    }

    static func roadConditionIcon(_ condition: String) -> String {
        switch condition.lowercased() {
        case "dry": return "sun.max.fill"
        case "wet": return "cloud.rain.fill"
        case "snowy", "snow covered", "snow": return "cloud.snow.fill"
        case "icy", "ice": return "snowflake"
        default: return "cloud.fill"
        }
    }

    static func weatherConditionIcon(_ condition: String) -> String {
        switch condition.lowercased() {
        case "clear": return "sun.max.fill"
        case "cloudy", "overcast": return "cloud.fill"
        case "rain", "rainy": return "cloud.rain.fill"
        case "snow", "snowy", "snowing": return "cloud.snow.fill"
        case "fog", "foggy": return "cloud.fog.fill"
        case "wind", "windy": return "wind"
        default: return "cloud.sun.fill"
        }
    }
}
