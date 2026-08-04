import SwiftUI

struct RoadConditionsBannerView: View {
    let conditions: [RoadCondition]

    var body: some View {
        if !conditions.isEmpty {
            VStack(spacing: 8) {
                ForEach(conditions) { condition in
                    RoadConditionRow(condition: condition)
                }
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 10)
            .transition(.opacity.combined(with: .move(edge: .top)))
        }
    }
}

private struct RoadConditionRow: View {
    let condition: RoadCondition

    var body: some View {
        HStack(spacing: 8) {
            Text(condition.RoadwayName)
                .font(.caption.weight(.semibold))
                .foregroundStyle(.white)
                .lineLimit(1)
                .layoutPriority(1)

            // Token chips — value only; color = meaning (no Road/Weather labels)
            if !condition.RoadCondition.isEmpty {
                TokenChip(
                    value: condition.RoadCondition,
                    color: ConditionColors.roadConditionColor(condition.RoadCondition)
                )
            }

            if !condition.WeatherCondition.isEmpty {
                TokenChip(
                    value: condition.WeatherCondition,
                    color: ConditionColors.weatherConditionColor(condition.WeatherCondition)
                )
            }

            // Restriction always shown — warn style when active, muted when none
            TokenChip(
                value: condition.hasRestriction ? condition.Restriction : "none",
                color: condition.hasRestriction ? ConditionColors.warningColor : .white.opacity(0.45),
                bold: condition.hasRestriction
            )

            Spacer(minLength: 4)

            Text(condition.timeAgo)
                .font(.caption2)
                .foregroundStyle(.white.opacity(0.35))
                .monospacedDigit()
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel(accessibilityText)
    }

    private var accessibilityText: String {
        var parts = ["\(condition.RoadwayName): Road \(condition.RoadCondition)"]
        if !condition.WeatherCondition.isEmpty {
            parts.append("Weather \(condition.WeatherCondition)")
        }
        if condition.hasRestriction {
            parts.append("restriction: \(condition.Restriction)")
        }
        parts.append("updated \(condition.timeAgo)")
        return parts.joined(separator: ", ")
    }
}

/// Colored pill; text is the value only (no field label).
private struct TokenChip: View {
    let value: String
    let color: Color
    var bold: Bool = false

    var body: some View {
        Text(value)
            .font(.caption2.weight(bold ? .bold : .medium))
            .foregroundStyle(color)
            .lineLimit(1)
            .padding(.horizontal, 7)
            .padding(.vertical, 3)
            .background(color.opacity(0.18), in: Capsule())
    }
}
