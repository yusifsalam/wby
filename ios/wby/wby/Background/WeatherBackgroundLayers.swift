import SwiftUI

struct SunLayer: View {
    let time: Float
    let size: CGSize

    var body: some View {
        Rectangle()
            .fill(ShaderLibrary.wby_sun(
                .float2(Float(size.width), Float(size.height)),
                .float(time)
            ))
            .allowsHitTesting(false)
    }
}

struct StarsLayer: View {
    let time: Float
    let size: CGSize

    var body: some View {
        Rectangle()
            .fill(ShaderLibrary.wby_stars(
                .float2(Float(size.width), Float(size.height)),
                .float(time)
            ))
            .allowsHitTesting(false)
    }
}

struct CloudsLayer: View {
    let time: Float
    let size: CGSize
    let coverage: Float
    let tint: SIMD4<Float>

    var body: some View {
        Rectangle()
            .fill(ShaderLibrary.wby_clouds(
                .float2(Float(size.width), Float(size.height)),
                .float(time),
                .float(coverage),
                .float4(tint.x, tint.y, tint.z, tint.w)
            ))
            .allowsHitTesting(false)
    }
}

struct RainLayer: View {
    let time: Float
    let size: CGSize
    let intensity: Float

    var body: some View {
        Rectangle()
            .fill(ShaderLibrary.wby_rain(
                .float2(Float(size.width), Float(size.height)),
                .float(time),
                .float(intensity)
            ))
            .allowsHitTesting(false)
    }
}

struct SnowLayer: View {
    let time: Float
    let size: CGSize
    let intensity: Float

    var body: some View {
        Rectangle()
            .fill(ShaderLibrary.wby_snow(
                .float2(Float(size.width), Float(size.height)),
                .float(time),
                .float(intensity)
            ))
            .allowsHitTesting(false)
    }
}

struct LightningLayer: View {
    let active: Bool
    @State private var opacity: Double = 0

    var body: some View {
        Rectangle()
            .fill(Color.white)
            .opacity(opacity)
            .allowsHitTesting(false)
            .task(id: active) {
                guard active else { opacity = 0; return }
                while !Task.isCancelled {
                    let wait = Double.random(in: 8.0...18.0)
                    try? await Task.sleep(for: .seconds(wait))
                    if Task.isCancelled { break }
                    await flash()
                }
            }
    }

    @MainActor
    private func flash() async {
        withAnimation(.easeIn(duration: 0.05)) { opacity = 0.45 }
        try? await Task.sleep(for: .seconds(0.10))
        withAnimation(.easeOut(duration: 0.25)) { opacity = 0 }
    }
}
