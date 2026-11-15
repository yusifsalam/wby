import SwiftUI

/// A row of forecast days, weekday initial over the date, with the selected
/// day circled and today in the accent color.
struct DayStripView: View {
    let details: [DayDetail]
    @Binding var selection: String

    var body: some View {
        HStack(spacing: 0) {
            ForEach(details) { detail in
                let selected = detail.id == selection
                Button {
                    withAnimation(.snappy) { selection = detail.id }
                } label: {
                    VStack(spacing: 6) {
                        Text(detail.weekdayInitial)
                            .font(.caption2.weight(.semibold))
                            .foregroundStyle(.secondary)
                        Text(detail.dayOfMonth)
                            .font(.callout.weight(.semibold))
                            .monospacedDigit()
                            .foregroundStyle(numberStyle(selected: selected, today: detail.isToday))
                            .frame(width: 32, height: 32)
                            .background {
                                if selected {
                                    Circle().fill(detail.isToday ? AnyShapeStyle(.tint) : AnyShapeStyle(.white))
                                }
                            }
                    }
                    .frame(maxWidth: .infinity)
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel(detail.fullDate)
                .accessibilityAddTraits(selected ? .isSelected : [])
            }
        }
    }

    private func numberStyle(selected: Bool, today: Bool) -> AnyShapeStyle {
        switch (selected, today) {
        case (true, true): AnyShapeStyle(.white)
        case (true, false): AnyShapeStyle(.black)
        case (false, true): AnyShapeStyle(.tint)
        case (false, false): AnyShapeStyle(.primary)
        }
    }
}

#Preview {
    @Previewable @State var selection = PreviewData.makeDaily()[0].date
    DayStripView(
        details: DayDetail.build(
            days: PreviewData.makeDaily(),
            hours: [],
            timeZone: TimeZone(identifier: "Europe/Helsinki")!
        ),
        selection: $selection
    )
    .padding()
}
