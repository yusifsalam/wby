import SwiftUI

struct HalfCard<AdditionalContent: View>: View {
    let title: String
    let icon: String
    let keyValue: String
    let keyValueUnit: String?
    let subtitle: String?
    let description: String?
    let additionalContent: AdditionalContent

    init(
        title: String,
        icon: String,
        keyValue: String,
        keyValueUnit: String? = nil,
        subtitle: String? = nil,
        description: String? = nil,
        @ViewBuilder additionalContent: () -> AdditionalContent
    ) {
        self.title = title
        self.icon = icon
        self.keyValue = keyValue
        self.keyValueUnit = keyValueUnit
        self.subtitle = subtitle
        self.description = description
        self.additionalContent = additionalContent()
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Label(title, systemImage: icon)
                .font(.caption)
                .foregroundStyle(.white.opacity(0.78))

            VStack(alignment: .leading) {
                HStack(alignment: .firstTextBaseline, spacing: 6) {
                    Text(keyValue)
                        .font(.largeTitle)
                        .minimumScaleFactor(0.75)
                        .foregroundStyle(.white)
                    if let keyValueUnit {
                        Text(keyValueUnit)
                            .font(.title)
                            .foregroundStyle(.white)
                    }
                }

                if let subtitle {
                    Text(subtitle)
                        .font(.system(size: 20, weight: .semibold))
                        .foregroundStyle(.white)
                }
            }

            additionalContent

            Spacer(minLength: 0)

            if let description {
                Text(description)
                    .font(.system(size: 14, weight: .regular))
                    .foregroundStyle(.white)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .weatherCard()
        .frame(maxHeight: 300)
    }
}

extension HalfCard where AdditionalContent == EmptyView {
    init(
        title: String,
        icon: String,
        keyValue: String,
        keyValueUnit: String? = nil,
        subtitle: String? = nil,
        description: String? = nil
    ) {
        self.init(
            title: title,
            icon: icon,
            keyValue: keyValue,
            keyValueUnit: keyValueUnit,
            subtitle: subtitle,
            description: description
        ) { EmptyView() }
    }
}

#Preview {
    ZStack {
        Color.blue.opacity(0.4).ignoresSafeArea()
        HStack(alignment: .top, spacing: 12) {
            HalfCard(
                title: "FEELS LIKE",
                icon: "thermometer.medium",
                keyValue: "-11°",
                description: "Wind can make it feel colder than the actual temperature."
            )
            HalfCard(
                title: "PRECIPITATION",
                icon: "drop.fill",
                keyValue: "0",
                keyValueUnit: "mm",
                subtitle: "0% chance",
                description: "No significant precipitation expected in the next days."
            )
        }
        .padding()
    }
}
