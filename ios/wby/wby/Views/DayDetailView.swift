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
                    panel(detail)
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

    // MARK: - Panel

    private func panel(_ detail: DayDetail) -> some View {
        VStack(alignment: .leading, spacing: 16) {
            header(detail)
            statGrid(detail.stats)
            if detail.hours.count >= 2 {
                DayDetailChart(detail: detail)
                    .weatherCard()
                hourTable(detail)
                    .weatherCard()
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

    private func header(_ detail: DayDetail) -> some View {
        HStack(spacing: 12) {
            Image(systemName: SmartSymbol.systemImageName(for: detail.day.symbol))
                .font(.system(size: 40))
                .symbolRenderingMode(.multicolor)
                .frame(width: 56, height: 56)
            VStack(alignment: .leading, spacing: 2) {
                Text(detail.longTitle)
                    .font(.title3)
                    .fontWeight(.bold)
                Text(detail.conditionDescription)
                    .foregroundStyle(.secondary)
            }
        }
    }

    private func statGrid(_ stats: [DayDetail.Stat]) -> some View {
        LazyVGrid(columns: [GridItem(.flexible(), spacing: 10), GridItem(.flexible(), spacing: 10)], spacing: 10) {
            ForEach(stats) { stat in
                VStack(alignment: .leading, spacing: 4) {
                    Text(stat.label)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    Text(stat.value)
                        .font(.title3)
                        .fontWeight(.semibold)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .weatherCard()
            }
        }
    }

    // MARK: - Hour table

    private func hourTable(_ detail: DayDetail) -> some View {
        let now = Date()
        let currentSlot = now.addingTimeInterval(-now.timeIntervalSince1970.truncatingRemainder(dividingBy: 3600))
        return VStack(spacing: 0) {
            HStack(spacing: Self.columnSpacing) {
                Text("Time").frame(width: Self.timeWidth, alignment: .leading)
                Color.clear.frame(width: Self.iconWidth, height: 1)
                Text("Temp").frame(width: Self.tempWidth, alignment: .leading)
                Text("Feels").frame(width: Self.tempWidth, alignment: .leading)
                Text("Rain").frame(width: Self.rainWidth, alignment: .leading)
                Text("Chance").frame(width: Self.chanceWidth, alignment: .leading)
                Text("Wind").frame(maxWidth: .infinity, alignment: .leading)
            }
            .font(.caption2)
            .fontWeight(.semibold)
            .foregroundStyle(.secondary)
            .padding(.bottom, 6)
            ForEach(detail.hours) { hour in
                hourRow(hour, in: detail, isPast: hour.time < currentSlot, isCurrent: hour.time == currentSlot)
            }
        }
    }

    private static let columnSpacing: CGFloat = 6
    private static let timeWidth: CGFloat = 26
    private static let iconWidth: CGFloat = 22
    private static let tempWidth: CGFloat = 34
    private static let rainWidth: CGFloat = 50
    private static let chanceWidth: CGFloat = 36

    private func hourRow(_ hour: HourlyForecast, in detail: DayDetail, isPast: Bool, isCurrent: Bool) -> some View {
        HStack(spacing: Self.columnSpacing) {
            Text(Self.hourLabel(hour.time, timeZone: detail.timeZone))
                .foregroundStyle(.secondary)
                .frame(width: Self.timeWidth, alignment: .leading)
            Image(systemName: SmartSymbol.systemImageName(for: WeatherSymbols.nightAdjusted(
                hour.symbol,
                coordinate: coordinate,
                at: hour.time,
                timeZone: detail.timeZone,
                elevationMeters: elevationMeters
            )))
            .symbolRenderingMode(.multicolor)
            .frame(width: Self.iconWidth)
            Text(DayDetail.formatTemp(hour.temperature))
                .fontWeight(.semibold)
                .frame(width: Self.tempWidth, alignment: .leading)
            Text(DayDetail.formatTemp(hour.feelsLike))
                .foregroundStyle(.secondary)
                .frame(width: Self.tempWidth, alignment: .leading)
            Text(DayDetail.formatPrecip(hour.precipitation1h))
                .frame(width: Self.rainWidth, alignment: .leading)
            Text(DayDetail.formatPercent(hour.pop))
                .foregroundStyle(.secondary)
                .frame(width: Self.chanceWidth, alignment: .leading)
            HStack(spacing: 3) {
                Text(DayDetail.formatSpeed(hour.windSpeed))
                if let direction = hour.windDirection {
                    Image(systemName: "arrow.up")
                        .font(.system(size: 9, weight: .bold))
                        .foregroundStyle(.secondary)
                        .rotationEffect(.degrees((direction + 180).truncatingRemainder(dividingBy: 360)))
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .font(.footnote)
        .monospacedDigit()
        .lineLimit(1)
        .minimumScaleFactor(0.8)
        .padding(.vertical, 5)
        .padding(.horizontal, 6)
        .opacity(isPast ? 0.45 : 1)
        .background {
            if isCurrent {
                RoundedRectangle(cornerRadius: 6, style: .continuous)
                    .fill(.primary.opacity(0.08))
            }
        }
        .padding(.horizontal, -6)
    }

    private static func hourLabel(_ date: Date, timeZone: TimeZone) -> String {
        let formatter = hourFormatter
        formatter.timeZone = timeZone
        return formatter.string(from: date)
    }

    private static let hourFormatter: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "HH"
        return f
    }()
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
