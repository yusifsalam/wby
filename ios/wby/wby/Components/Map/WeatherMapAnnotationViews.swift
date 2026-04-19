import MapKit
import SwiftUI
import UIKit

private struct FavoriteWeatherPinBubbleView: View {
    let currentText: String
    let rangeText: String

    var body: some View {
        VStack(spacing: 2) {
            VStack(spacing: 0) {
                Text(currentText)
                    .font(.system(size: 19, weight: .semibold))
                    .frame(maxWidth: .infinity)
                    .padding(.top, 3)

                Text(rangeText)
                    .font(.system(size: 10, weight: .medium, design: .monospaced))
                    .foregroundStyle(Color.white.opacity(0.88))
                    .frame(maxWidth: .infinity)
            }
            .frame(width: 86, height: 44)
            .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 14, style: .continuous))
            .overlay {
                RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .stroke(Color.white.opacity(0.26), lineWidth: 0.5)
            }

            Circle()
                .fill(Color(red: 0.21, green: 0.69, blue: 1.0))
                .overlay {
                    Circle().stroke(Color.white.opacity(0.9), lineWidth: 1.0)
                }
                .frame(width: 8, height: 8)
        }
        .foregroundStyle(.white)
        .frame(width: 86, height: 54)
    }
}

private struct PreviewPinBubbleView: View {
    let placeName: String
    let conditionText: String
    let symbolSystemName: String?
    let tempText: String
    let rangeText: String
    let onTap: (() -> Void)?

    var body: some View {
        VStack(spacing: 2) {
            VStack(spacing: 0) {
                Text(placeName)
                    .font(.system(size: 13, weight: .semibold))
                    .lineLimit(1)
                    .frame(maxWidth: .infinity)
                    .padding(.top, 6)

                HStack(spacing: 4) {
                    if let symbolSystemName {
                        Image(systemName: symbolSystemName)
                            .font(.system(size: 12, weight: .regular))
                    }
                    Text(conditionText)
                        .font(.system(size: 12, weight: .regular))
                        .lineLimit(1)
                }
                .foregroundStyle(Color.white.opacity(0.88))
                .frame(maxWidth: .infinity)
                .padding(.top, 2)

                Text(tempText)
                    .font(.system(size: 28, weight: .semibold))
                    .frame(maxWidth: .infinity)
                    .padding(.top, 2)

                Text(rangeText)
                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                    .foregroundStyle(Color.white.opacity(0.88))
                    .frame(maxWidth: .infinity)
            }
            .frame(width: 160, height: 92)
            .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 14, style: .continuous))
            .overlay {
                RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .stroke(Color.white.opacity(0.26), lineWidth: 0.5)
            }
            .onTapGesture {
                onTap?()
            }

            Circle()
                .fill(Color(red: 0.21, green: 0.69, blue: 1.0))
                .overlay {
                    Circle().stroke(Color.white.opacity(0.9), lineWidth: 1.0)
                }
                .frame(width: 8, height: 8)
        }
        .foregroundStyle(.white)
        .frame(width: 160, height: 104)
    }
}

final class BubbleAnnotationView: MKAnnotationView {
    static let favoriteReuseID = "FavoriteWeatherPin"
    static let previewReuseID = "PreviewPin"

    private var onPreviewTap: (() -> Void)?
    private var hostingController: UIHostingController<AnyView>?

    override init(annotation: (any MKAnnotation)?, reuseIdentifier: String?) {
        super.init(annotation: annotation, reuseIdentifier: reuseIdentifier)
        setupView()
    }

    required init?(coder: NSCoder) {
        super.init(coder: coder)
        setupView()
    }

    override func prepareForReuse() {
        super.prepareForReuse()
        onPreviewTap = nil
        isHidden = false
        alpha = 1
        setHostedContent(AnyView(EmptyView()), isUserInteractionEnabled: false)
    }

    func configureFavorite(with annotation: FavoritePinAnnotation) {
        let weather = annotation.weather
        let currentText = TemperatureText.value(weather?.current)
        let rangeText = TemperatureText.range(low: weather?.low, high: weather?.high)
        centerOffset = CGPoint(x: 0, y: -26)
        bounds = CGRect(x: 0, y: 0, width: 86, height: 54)
        setHostedContent(
            AnyView(
                FavoriteWeatherPinBubbleView(
                    currentText: currentText,
                    rangeText: rangeText
                )
            ),
            isUserInteractionEnabled: false
        )

        isHidden = !annotation.isVisibleAtCurrentZoom
        alpha = annotation.isVisibleAtCurrentZoom ? 1 : 0
        accessibilityLabel = annotation.title ?? "Favorite location"
        accessibilityValue = "\(currentText), \(rangeText)"
    }

    func configurePreview(with annotation: PreviewPinAnnotation, onTap: (() -> Void)? = nil) {
        if let onTap {
            onPreviewTap = onTap
        }

        let placeName = annotation.placeName ?? "—"
        var conditionText = "Loading…"
        var symbolSystemName: String?
        var tempText = "--°"
        var rangeText = "L --°  H --°"

        switch annotation.loadState {
        case .loading:
            break
        case .failed:
            conditionText = "Weather unavailable"
        case .loaded:
            let weather = annotation.weather
            conditionText = weather?.conditionText ?? "—"
            if let code = weather?.symbolCode {
                symbolSystemName = SmartSymbol.systemImageName(for: code)
            }
            tempText = TemperatureText.value(weather?.current)
            rangeText = TemperatureText.range(low: weather?.low, high: weather?.high)
        }

        centerOffset = CGPoint(x: 0, y: -54)
        bounds = CGRect(x: 0, y: 0, width: 160, height: 104)
        setHostedContent(
            AnyView(
                PreviewPinBubbleView(
                    placeName: placeName,
                    conditionText: conditionText,
                    symbolSystemName: symbolSystemName,
                    tempText: tempText,
                    rangeText: rangeText,
                    onTap: onPreviewTap
                )
            ),
            isUserInteractionEnabled: true
        )

        accessibilityLabel = placeName
        accessibilityValue = "\(tempText), \(conditionText)"
    }

    private func setupView() {
        backgroundColor = .clear
        canShowCallout = false
        collisionMode = .none
        displayPriority = .required
    }

    private func setHostedContent(_ rootView: AnyView, isUserInteractionEnabled: Bool) {
        if let hostingController {
            hostingController.rootView = rootView
            hostingController.view.isUserInteractionEnabled = isUserInteractionEnabled
            hostingController.view.frame = bounds
            return
        }

        let controller = UIHostingController(rootView: rootView)
        controller.view.backgroundColor = .clear
        controller.view.isUserInteractionEnabled = isUserInteractionEnabled
        controller.view.frame = bounds
        controller.view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
        addSubview(controller.view)
        hostingController = controller
    }
}

#Preview("Favorite Pin Bubble") {
    ZStack {
        LinearGradient(
            colors: [Color.cyan.opacity(0.45), Color.blue.opacity(0.75)],
            startPoint: .topLeading,
            endPoint: .bottomTrailing
        )
        .ignoresSafeArea()

        FavoriteWeatherPinBubbleView(
            currentText: "9°",
            rangeText: "L 4°  H 12°"
        )
        .padding(20)
    }
}

#Preview("Preview Pin Bubble Loaded") {
    ZStack {
        LinearGradient(
            colors: [Color.teal.opacity(0.4), Color.indigo.opacity(0.85)],
            startPoint: .top,
            endPoint: .bottom
        )
        .ignoresSafeArea()

        PreviewPinBubbleView(
            placeName: "Helsinki",
            conditionText: "Partly cloudy",
            symbolSystemName: "cloud.sun.fill",
            tempText: "8°",
            rangeText: "L 3°  H 11°",
            onTap: nil
        )
        .padding(20)
    }
}

#Preview("Preview Pin Bubble Loading") {
    ZStack {
        LinearGradient(
            colors: [Color.gray.opacity(0.45), Color.black.opacity(0.85)],
            startPoint: .top,
            endPoint: .bottom
        )
        .ignoresSafeArea()

        PreviewPinBubbleView(
            placeName: "60.17, 24.94",
            conditionText: "Loading…",
            symbolSystemName: nil,
            tempText: "--°",
            rangeText: "L --°  H --°",
            onTap: nil
        )
        .padding(20)
    }
}
