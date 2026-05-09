import SwiftUI

struct TimeScrubberView: View {
    @ObservedObject var viewModel: WeatherMapViewModel
    private let timeZone = TimeZone(identifier: "Europe/Helsinki") ?? .current

    private var pastHours: Int { WeatherMapViewModel.scrubberPastHours }
    private var futureHours: Int { WeatherMapViewModel.scrubberFutureHours }

    var body: some View {
        VStack(spacing: 6) {
            HStack(spacing: 12) {
                playPauseButton
                slider
                timeLabel
                    .frame(width: 64, alignment: .trailing)
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 10)
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 14))
    }

    private var playPauseButton: some View {
        Button {
            viewModel.togglePlayback()
        } label: {
            Image(systemName: viewModel.isPlaying ? "pause.fill" : "play.fill")
                .font(.system(size: 13, weight: .bold))
                .frame(width: 28, height: 28)
                .background(.ultraThinMaterial, in: Circle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(viewModel.isPlaying ? "Pause time playback" : "Play time playback")
    }

    private var slider: some View {
        let binding = Binding<Double>(
            get: { hoursFromNow(viewModel.selectedTime) },
            set: { newHours in
                if viewModel.isPlaying { viewModel.togglePlayback() }
                let target = Date().addingTimeInterval(newHours * 3600)
                viewModel.setSelectedTime(target)
            }
        )
        return ZStack(alignment: .top) {
            ticksRow
                .padding(.top, 22)
            Slider(
                value: binding,
                in: Double(-pastHours)...Double(futureHours)
            )
            .tint(.teal)
        }
        .frame(height: 36)
    }

    private var ticksRow: some View {
        GeometryReader { proxy in
            let width = proxy.size.width
            let total = pastHours + futureHours
            Canvas { context, size in
                guard total > 0 else { return }
                for offset in 0...total {
                    let xRatio = Double(offset) / Double(total)
                    let x = xRatio * Double(width)
                    let isNow = offset == pastHours
                    let height: CGFloat = isNow ? 8 : 4
                    let rect = CGRect(x: x - 0.5, y: 0, width: 1, height: height)
                    let color: Color = isNow ? .teal : .secondary.opacity(0.5)
                    context.fill(Path(rect), with: .color(color))
                }
            }
            .frame(width: width, height: 8)
        }
        .frame(height: 8)
        .allowsHitTesting(false)
    }

    private var timeLabel: some View {
        VStack(alignment: .trailing, spacing: 1) {
            Text(absoluteLabel(for: viewModel.selectedTime))
                .font(.caption.bold())
                .monospacedDigit()
            Text(relativeLabel(for: viewModel.selectedTime))
                .font(.caption2)
                .foregroundStyle(.secondary)
                .monospacedDigit()
        }
    }

    private func hoursFromNow(_ date: Date) -> Double {
        date.timeIntervalSince(Date()) / 3600
    }

    private func absoluteLabel(for date: Date) -> String {
        let f = DateFormatter()
        f.dateStyle = .none
        f.timeStyle = .short
        f.timeZone = timeZone
        return f.string(from: date)
    }

    private func relativeLabel(for date: Date) -> String {
        let hours = Int(hoursFromNow(date).rounded())
        if hours == 0 { return "Now" }
        if hours > 0 { return "+\(hours)h" }
        return "\(hours)h"
    }
}

private struct TimeScrubberPreviewHost: View {
    @StateObject private var viewModel: WeatherMapViewModel

    init() {
        let weatherService = WeatherService()
        _viewModel = StateObject(
            wrappedValue: WeatherMapViewModel(
                overlayService: MapOverlayService(weatherService: weatherService),
                weatherService: weatherService,
                networkEnabled: false
            )
        )
    }

    var body: some View {
        TimeScrubberView(viewModel: viewModel)
            .padding()
            .background(Color.black.opacity(0.4))
    }
}

#Preview("Time scrubber") {
    TimeScrubberPreviewHost()
}
