import SwiftUI

struct FullCard<Visual: View>: View {
    let title: String
    let icon: String
    let rows: [(String, String)]
    let visual: Visual

    init(
        title: String,
        icon: String,
        rows: [(String, String)],
        @ViewBuilder visual: () -> Visual
    ) {
        self.title = title
        self.icon = icon
        self.rows = rows
        self.visual = visual()
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Label(title, systemImage: icon)
                .font(.caption)
                .foregroundStyle(.secondary)

            HStack(alignment: .center, spacing: 22) {
                VStack(spacing: 0) {
                    ForEach(Array(rows.enumerated()), id: \.offset) { index, row in
                        if index > 0 {
                            Divider().overlay(Color.primary.opacity(0.16))
                        }
                        HStack {
                            Text(row.0)
                                .font(.subheadline)
                                .foregroundStyle(.primary)
                            Spacer()
                            Text(row.1)
                                .font(.subheadline)
                                .foregroundStyle(.secondary)
                        }
                        .padding(.vertical, 13)
                    }
                }
                .frame(maxWidth: .infinity)

                visual
            }
        }
        .weatherCard()
        .frame(minHeight: 200, maxHeight: 300)
    }
}

#Preview {
    ZStack {
        Color.blue.opacity(0.45).ignoresSafeArea()
        FullCard(
            title: "WIND",
            icon: "wind",
            rows: [
                ("Wind", "6 m/s"),
                ("Gusts", "9 m/s"),
                ("Direction", "293° NW"),
            ]
        ) {
            Circle()
                .fill(.white.opacity(0.15))
                .frame(width: 136, height: 136)
                .overlay(
                    Text("visual")
                        .font(.caption)
                        .foregroundStyle(.white.opacity(0.6))
                )
        }
        .padding()
    }
}
