import SwiftUI

struct RoadConditionsBannerView: View {
    let conditions: [RoadCondition]

    var body: some View {
        if !conditions.isEmpty {
            VStack(spacing: 6) {
                ForEach(conditions) { condition in
                    RoadConditionRow(condition: condition)
                }
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 8)
            .transition(.opacity.combined(with: .move(edge: .top)))
        }
    }
}

private struct RoadConditionRow: View {
    let condition: RoadCondition

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(condition.RoadwayName)
                .font(.caption)
                .fontWeight(.semibold)
                .foregroundStyle(.white)
                .lineLimit(1)

            HStack(spacing: 6) {
                // Road condition badge
                ConditionBadge(
                    label: "Road",
                    value: condition.RoadCondition,
                    color: ConditionColors.roadConditionColor(condition.RoadCondition)
                )

                // Weather condition badge
                if !condition.WeatherCondition.isEmpty {
                    ConditionBadge(
                        label: "Weather",
                        value: condition.WeatherCondition,
                        color: .white.opacity(0.7)
                    )
                }

                // Restriction badge
                if condition.hasRestriction {
                    ConditionBadge(
                        label: "Restriction",
                        value: condition.Restriction,
                        color: .yellow,
                        isWarning: true
                    )
                }

                Spacer()

                Text(condition.timeAgo)
                    .font(.system(size: 10))
                    .foregroundStyle(.white.opacity(0.4))
            }
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 8)
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 10))
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(condition.RoadwayName): Road \(condition.RoadCondition), Weather \(condition.WeatherCondition)\(condition.hasRestriction ? ", restriction: \(condition.Restriction)" : "")")
    }
}

private struct ConditionBadge: View {
    let label: String
    let value: String
    let color: Color
    var isWarning: Bool = false

    var body: some View {
        HStack(spacing: 2) {
            Text("\(label):")
                .font(.system(size: 10))
                .foregroundStyle(.white.opacity(0.5))
            Text(value)
                .font(.system(size: 10, weight: isWarning ? .bold : .medium))
                .foregroundStyle(color)
                .lineLimit(1)
        }
    }
}
