import SwiftUI

/// A titled card in a detail sheet.
struct DetailSection<Content: View>: View {
    let title: String
    @ViewBuilder let content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title)
                .font(.headline)
            content
        }
        .weatherCard()
    }
}

#Preview {
    DetailSection(title: "Daily Summary") {
        Text("Today, wind speeds are 4 to 9 m/s, with gusts up to 17 m/s.")
            .font(.subheadline)
    }
    .padding()
}
