import SwiftUI

enum ConditionColors {
    // Match website: green=#10b981, blue=#3b82f6, lightBlue=#60a5fa, indigo=#6366f1, purple=#8b5cf6
    static func roadConditionColor(_ condition: String) -> Color {
        switch condition.lowercased() {
        case "dry": return Color(red: 0.063, green: 0.725, blue: 0.506) // #10b981
        case "wet": return Color(red: 0.231, green: 0.510, blue: 0.965) // #3b82f6
        case "snowy", "snow covered", "snow": return Color(red: 0.376, green: 0.647, blue: 0.980) // #60a5fa
        case "icy", "ice": return Color(red: 0.388, green: 0.400, blue: 0.945) // #6366f1
        case "slush", "slushy": return Color(red: 0.545, green: 0.361, blue: 0.965) // #8b5cf6
        default: return .secondary
        }
    }

    // Match website: amber=#f59e0b, slate=#64748b, blue=#3b82f6, lightBlue=#60a5fa
    static func weatherConditionColor(_ condition: String) -> Color {
        switch condition.lowercased() {
        case "fair", "clear": return Color(red: 0.961, green: 0.620, blue: 0.043) // #f59e0b
        case "cloudy", "overcast": return Color(red: 0.392, green: 0.455, blue: 0.545) // #64748b
        case "rain", "rainy": return Color(red: 0.231, green: 0.510, blue: 0.965) // #3b82f6
        case "snow", "snowy", "snowing": return Color(red: 0.376, green: 0.647, blue: 0.980) // #60a5fa
        default: return .secondary
        }
    }

    static let warningColor = Color(red: 0.961, green: 0.620, blue: 0.043) // #f59e0b
}
