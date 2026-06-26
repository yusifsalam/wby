import SwiftUI

struct TimeScrubberView: View {
    let viewModel: WeatherMapViewModel
    private let timeZone = TimeZone(identifier: "Europe/Helsinki") ?? .current

    private var pastSteps: Int { viewModel.scrubberPastSteps }
    private var futureSteps: Int { viewModel.scrubberFutureSteps }
    private var stepSeconds: TimeInterval { viewModel.scrubberStepSeconds }

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
        .glassEffect(in: RoundedRectangle(cornerRadius: 14))
    }

    private var playPauseButton: some View {
        Button {
            viewModel.togglePlayback()
        } label: {
            Image(systemName: viewModel.isPlaying ? "pause.fill" : "play.fill")
                .font(.system(size: 13, weight: .bold))
                .frame(width: 28, height: 28)
                .background(Color.primary.opacity(0.1), in: Circle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(viewModel.isPlaying ? "Pause time playback" : "Play time playback")
    }

    private var slider: some View {
        let binding = Binding<Double>(
            get: { stepsFromNow(viewModel.selectedTime) },
            set: { newSteps in
                if viewModel.isPlaying { viewModel.togglePlayback() }
                let target = Date().addingTimeInterval(newSteps * stepSeconds)
                viewModel.setSelectedTime(target)
            }
        )
        return ZStack(alignment: .top) {
            ticksRow
                .padding(.top, 22)
            Slider(
                value: binding,
                in: Double(-pastSteps)...Double(futureSteps)
            )
            .tint(.teal)
        }
        .frame(height: 36)
    }

    private var ticksRow: some View {
        // Cap drawn ticks so dense step grids (e.g. 5-min × 48 steps) don't
        // turn into a solid bar. We always draw the "now" marker plus a
        // sparser grid that scales with available width.
        GeometryReader { proxy in
            let width = proxy.size.width
            let total = pastSteps + futureSteps
            let majorEvery = tickStride(totalSteps: total)
            Canvas { context, size in
                guard total > 0 else { return }
                for offset in 0...total {
                    let isNow = offset == pastSteps
                    let isMajor = isNow || offset % majorEvery == 0
                    if !isMajor { continue }
                    let xRatio = Double(offset) / Double(total)
                    let x = xRatio * Double(width)
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

    private func stepsFromNow(_ date: Date) -> Double {
        date.timeIntervalSince(Date()) / stepSeconds
    }

    private func tickStride(totalSteps: Int) -> Int {
        // Aim for at most ~24 visible major ticks regardless of granularity.
        if totalSteps <= 24 { return 1 }
        return max(1, Int((Double(totalSteps) / 24.0).rounded(.up)))
    }

    private func absoluteLabel(for date: Date) -> String {
        let f = DateFormatter()
        f.dateStyle = .none
        f.timeStyle = .short
        f.timeZone = timeZone
        return f.string(from: date)
    }

    private func relativeLabel(for date: Date) -> String {
        let delta = date.timeIntervalSince(Date())
        if abs(delta) < 60 { return "Now" }
        let absMinutes = Int(abs(delta) / 60)
        let sign = delta >= 0 ? "+" : "-"
        if absMinutes >= 60 {
            let h = absMinutes / 60
            let m = absMinutes % 60
            if m == 0 { return "\(sign)\(h)h" }
            return "\(sign)\(h)h\(m)m"
        }
        return "\(sign)\(absMinutes)m"
    }
}

private struct TimeScrubberPreviewHost: View {
    @State private var viewModel: WeatherMapViewModel

    init() {
        let weatherService = WeatherService()
        _viewModel = State(
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
