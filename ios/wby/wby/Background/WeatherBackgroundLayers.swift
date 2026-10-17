import SwiftUI

@Animatable
struct SkyLayer: View {
    var cumulus: Float
    var deck: Float
    var cirrus: Float
    var absorption: Float
    var sun: Float
    var night: Float
    var lit: Color.Resolved
    var shadow: Color.Resolved
    @AnimatableIgnored var size: CGSize
    @AnimatableIgnored var downscale: CGFloat
    @AnimatableIgnored var time: Float
    @AnimatableIgnored var drift: SIMD2<Float>
    @AnimatableIgnored var sunFade: Float
    @AnimatableIgnored var flash: Float
    @AnimatableIgnored var flashX: Float

    init(
        style: SkyStyle,
        size: CGSize,
        downscale: CGFloat,
        time: Float,
        drift: SIMD2<Float>,
        sunFade: Float,
        flash: Float,
        flashX: Float
    ) {
        cumulus = style.cumulus
        deck = style.deck
        cirrus = style.cirrus
        absorption = style.absorption
        sun = style.sun
        night = style.night
        lit = style.lit
        shadow = style.shadow
        self.size = size
        self.downscale = max(downscale, 1)
        self.time = time
        self.drift = drift
        self.sunFade = sunFade
        self.flash = flash
        self.flashX = flashX
    }

    var body: some View {
        let occlusion = sun * sunFade > 0.001
            ? SkyNoise.sunOcclusion(size: size, time: time, drift: drift, coverage: cumulus, deck: deck)
            : 0
        let reduced = CGSize(width: size.width / downscale, height: size.height / downscale)
        Rectangle()
            .fill(ShaderLibrary.wby_sky(
                .float2(Float(reduced.width), Float(reduced.height)),
                .float2(Float(size.width), Float(size.height)),
                .float(time),
                .float2(drift.x, drift.y),
                .float4(cumulus, deck, cirrus, absorption),
                .float4(sun, sunFade, night, occlusion),
                .float4(lit.red, lit.green, lit.blue, 1),
                .float4(shadow.red, shadow.green, shadow.blue, 1),
                .float4(flash, flashX, 0, 0)
            ))
            .frame(width: reduced.width, height: reduced.height)
            .drawingGroup()
            .scaleEffect(downscale, anchor: .topLeading)
            .frame(width: size.width, height: size.height, alignment: .topLeading)
    }
}

@Animatable
struct StarsLayer: View {
    var amount: Float
    @AnimatableIgnored var size: CGSize
    @AnimatableIgnored var time: Float

    var body: some View {
        Rectangle()
            .fill(ShaderLibrary.wby_stars(
                .float2(Float(size.width), Float(size.height)),
                .float(time),
                .float(amount)
            ))
    }
}

@Animatable
struct RainLayer: View {
    var intensity: Float
    var slant: Float
    @AnimatableIgnored var size: CGSize
    @AnimatableIgnored var phase: Float

    var body: some View {
        Rectangle()
            .fill(ShaderLibrary.wby_rain(
                .float2(Float(size.width), Float(size.height)),
                .float(phase),
                .float(intensity),
                .float(slant)
            ))
    }
}

@Animatable
struct SnowLayer: View {
    var intensity: Float
    var wind: Float
    @AnimatableIgnored var size: CGSize
    @AnimatableIgnored var time: Float
    @AnimatableIgnored var fall: SIMD2<Float>

    var body: some View {
        Rectangle()
            .fill(ShaderLibrary.wby_snow(
                .float2(Float(size.width), Float(size.height)),
                .float(time),
                .float2(fall.x, fall.y),
                .float(intensity),
                .float(wind)
            ))
    }
}
