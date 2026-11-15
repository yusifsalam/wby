import CoreLocation
import SwiftUI

struct DayDetailView: View {
    let details: [DayDetail]
    let isLoadingHours: Bool
    var coordinate: CLLocationCoordinate2D = CLLocationCoordinate2D(latitude: 60.1699, longitude: 24.9384)
    var elevationMeters: Double = 0

    @State private var selection: String

    init(
        details: [DayDetail],
        initialDate: String,
        isLoadingHours: Bool,
        coordinate: CLLocationCoordinate2D = CLLocationCoordinate2D(latitude: 60.1699, longitude: 24.9384),
        elevationMeters: Double = 0
    ) {
        self.details = details
        self.isLoadingHours = isLoadingHours
        self.coordinate = coordinate
        self.elevationMeters = elevationMeters
        self._selection = State(initialValue: initialDate)
    }

    private var index: Int {
        details.firstIndex { $0.id == selection } ?? 0
    }

    var body: some View {
        TabView(selection: $selection) {
            ForEach(details) { detail in
                ScrollView {
                    DayPanelView(
                        detail: detail,
                        isLoadingHours: isLoadingHours,
                        coordinate: coordinate,
                        elevationMeters: elevationMeters
                    )
                    .padding()
                }
                .tag(detail.id)
            }
        }
        .tabViewStyle(.page(indexDisplayMode: .never))
        .navigationTitle(details.indices.contains(index) ? details[index].shortTitle : "Forecast")
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItemGroup(placement: .topBarLeading) {
                Button {
                    move(by: -1)
                } label: {
                    Image(systemName: "chevron.left")
                }
                .disabled(index == 0)
                Button {
                    move(by: 1)
                } label: {
                    Image(systemName: "chevron.right")
                }
                .disabled(index >= details.count - 1)
            }
        }
    }

    private func move(by delta: Int) {
        let target = index + delta
        guard details.indices.contains(target) else { return }
        withAnimation { selection = details[target].id }
    }
}

#Preview {
    let timeZone = TimeZone(identifier: "Europe/Helsinki")!
    let days = PreviewData.makeDaily()
    NavigationStack {
        DayDetailView(
            details: DayDetail.build(days: days, hours: PreviewData.makeFullDay(), timeZone: timeZone),
            initialDate: days[0].date,
            isLoadingHours: false
        )
    }
}
