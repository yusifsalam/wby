import SwiftUI

/// The shared frame of the per-day detail sheets: a day strip, the selected
/// date with previous/next buttons, and the day's content once its hours are
/// loaded.
struct DaySheetContainer<Content: View>: View {
    let details: [DayDetail]
    let isLoadingHours: Bool
    @Binding var selection: String
    @ViewBuilder let content: (DayDetail) -> Content

    private var index: Int {
        details.firstIndex { $0.id == selection } ?? 0
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                DayStripView(details: details, selection: $selection)
                if details.indices.contains(index) {
                    let detail = details[index]
                    dateHeader(detail)
                    Divider()
                    if detail.hours.count >= 2 {
                        content(detail)
                    } else if isLoadingHours {
                        ProgressView("Loading hours…")
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 24)
                    } else {
                        Text("Hourly detail isn't available this far ahead.")
                            .font(.footnote)
                            .foregroundStyle(.secondary)
                    }
                }
            }
            .padding()
        }
    }

    private func dateHeader(_ detail: DayDetail) -> some View {
        HStack {
            stepButton("chevron.left", label: "Previous day", delta: -1)
            Spacer()
            Text(detail.fullDate)
                .font(.headline)
                .lineLimit(1)
                .minimumScaleFactor(0.8)
            Spacer()
            stepButton("chevron.right", label: "Next day", delta: 1)
        }
    }

    private func stepButton(_ systemImage: String, label: String, delta: Int) -> some View {
        Button {
            move(by: delta)
        } label: {
            Image(systemName: systemImage)
                .font(.subheadline.weight(.semibold))
                .frame(width: 32, height: 32)
                .background(.fill.tertiary, in: Circle())
        }
        .buttonStyle(.plain)
        .disabled(!details.indices.contains(index + delta))
        .accessibilityLabel(label)
    }

    private func move(by delta: Int) {
        let target = index + delta
        guard details.indices.contains(target) else { return }
        withAnimation(.snappy) { selection = details[target].id }
    }
}

#Preview {
    @Previewable @State var selection = PreviewData.makeDaily()[0].date
    DaySheetContainer(
        details: DayDetail.build(
            days: PreviewData.makeDaily(),
            hours: PreviewData.makeFullDay(),
            timeZone: TimeZone(identifier: "Europe/Helsinki")!
        ),
        isLoadingHours: false,
        selection: $selection
    ) { detail in
        Text(detail.conditionDescription)
    }
}
