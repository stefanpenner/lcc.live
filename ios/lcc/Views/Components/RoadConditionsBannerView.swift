import SwiftUI

struct RoadConditionsBannerView: View {
    let conditions: [RoadCondition]

    var body: some View {
        if !conditions.isEmpty {
            VStack(spacing: 14) {
                ForEach(conditions) { condition in
                    RoadConditionRow(condition: condition)
                }
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 18)
            .transition(.opacity.combined(with: .move(edge: .top)))
        }
    }
}

private struct RoadConditionRow: View {
    let condition: RoadCondition

    var body: some View {
        VStack(alignment: .leading, spacing: 7) {
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                Text(condition.RoadwayName)
                    .font(.callout)
                    .fontWeight(.semibold)
                    .foregroundStyle(.white)
                    .lineLimit(1)

                Spacer(minLength: 4)

                Text(condition.timeAgo)
                    .font(.caption)
                    .foregroundStyle(.white.opacity(0.35))
            }

            HStack(spacing: 16) {
                badge(label: "Road", value: condition.RoadCondition,
                      color: ConditionColors.roadConditionColor(condition.RoadCondition))

                if !condition.WeatherCondition.isEmpty {
                    badge(label: "Wthr", value: condition.WeatherCondition,
                          color: ConditionColors.weatherConditionColor(condition.WeatherCondition))
                }

                if condition.hasRestriction {
                    badge(label: "Restriction", value: condition.Restriction,
                          color: ConditionColors.warningColor, bold: true)
                }
            }
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(condition.RoadwayName): Road \(condition.RoadCondition), Weather \(condition.WeatherCondition)\(condition.hasRestriction ? ", restriction: \(condition.Restriction)" : "")")
    }

    @ViewBuilder
    private func badge(label: String, value: String, color: Color, bold: Bool = false) -> some View {
        HStack(spacing: 5) {
            Text(label)
                .font(.caption)
                .foregroundStyle(.white.opacity(0.4))
            Text(value)
                .font(.subheadline)
                .fontWeight(bold ? .semibold : .medium)
                .foregroundStyle(color)
                .lineLimit(1)
        }
    }
}
