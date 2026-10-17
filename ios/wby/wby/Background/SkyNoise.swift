import CoreGraphics
import Foundation

/// CPU mirror of the cloud volume in WeatherBackgroundShaders.metal.
enum SkyNoise {
    static let pitch: Float = 0.50
    static let tanHalfFov: Float = 0.80
    static let layerBottom: Float = 1.0
    static let sunUV = SIMD2<Float>(0.20, 0.04)

    static func hash3(_ x: Int32, _ y: Int32, _ z: Int32) -> Float {
        var n = (UInt32(bitPattern: x) &* 1_597_334_673)
            ^ (UInt32(bitPattern: y) &* 3_812_015_801)
            ^ (UInt32(bitPattern: z) &* 2_798_796_415)
        n = (n ^ (n >> 15)) &* 2_246_822_519
        n = (n ^ (n >> 13)) &* 3_266_489_917
        return Float(n ^ (n >> 16)) * (1.0 / 4_294_967_295.0)
    }

    static func noise3(_ p: SIMD3<Float>) -> Float {
        let i = p.rounded(.down)
        let f = p - i
        let u = f * f * (3 - 2 * f)
        let x = Int32(i.x)
        let y = Int32(i.y)
        let z = Int32(i.z)
        let x00 = mix(hash3(x, y, z), hash3(x + 1, y, z), u.x)
        let x10 = mix(hash3(x, y + 1, z), hash3(x + 1, y + 1, z), u.x)
        let x01 = mix(hash3(x, y, z + 1), hash3(x + 1, y, z + 1), u.x)
        let x11 = mix(hash3(x, y + 1, z + 1), hash3(x + 1, y + 1, z + 1), u.x)
        return mix(mix(x00, x10, u.y), mix(x01, x11, u.y), u.z)
    }

    static func rotate3(_ p: SIMD3<Float>) -> SIMD3<Float> {
        SIMD3(
            -0.80 * p.y - 0.60 * p.z,
            0.80 * p.x + 0.36 * p.y - 0.48 * p.z,
            0.60 * p.x - 0.48 * p.y + 0.64 * p.z
        )
    }

    static func viewRay(_ uv: SIMD2<Float>, aspect: Float) -> SIMD3<Float> {
        let ndc = SIMD2((uv.x - 0.5) * 2 * aspect, (0.5 - uv.y) * 2) * tanHalfFov
        let s = sin(pitch)
        let c = cos(pitch)
        let v = SIMD3(ndc.x, s + c * ndc.y, c - s * ndc.y)
        return v / (v * v).sum().squareRoot()
    }

    static func layerTop(deck: Float) -> Float {
        layerBottom + mix(0.45, 0.65, deck)
    }

    static func density(
        _ p: SIMD3<Float>,
        coverage: Float,
        deck: Float,
        top: Float,
        offset: SIMD3<Float>,
        octaves: Int
    ) -> Float {
        let h = (p.y - layerBottom) / (top - layerBottom)
        let cumulusProfile = smoothstep(0, 0.12, h) * (1 - smoothstep(0.25, 1, h))
        let deckProfile = smoothstep(0, 0.10, h) * (1 - smoothstep(0.55, 1, h))
        let profile = mix(cumulusProfile, deckProfile, deck)
        guard profile > 0 else { return 0 }

        var q = p * mix(3.0, 3.8, deck) + offset
        var f = 0.5 * noise3(q)
        q = rotate3(q) * 2.02
        f += 0.25 * noise3(q)
        var weight: Float = 0.75
        if octaves > 2 {
            q = rotate3(q) * 2.03
            f += 0.125 * noise3(q)
            weight += 0.125
        }
        f /= weight

        let threshold = mix(0.78, 0.30, coverage)
        return min(max((f * profile - threshold) / (1 - threshold) * 3.2, 0), 1)
    }

    static func cloudAlpha(
        along rd: SIMD3<Float>,
        time: Float,
        drift: SIMD2<Float>,
        coverage: Float,
        deck: Float
    ) -> Float {
        guard rd.y >= 0.03 else { return 0 }
        let top = layerTop(deck: deck)
        let t0 = layerBottom / rd.y
        guard t0 <= 9 else { return 0 }
        let t1 = min(top / rd.y, 9)
        let steps = 12
        let dt = (t1 - t0) / Float(steps)
        let offset = SIMD3(-drift.x, time * 0.003, -drift.y)
        var t = t0 + dt * 0.5
        var alpha: Float = 0
        for _ in 0..<steps {
            let den = density(rd * t, coverage: coverage, deck: deck, top: top, offset: offset, octaves: 3)
            if den > 0.003 {
                let a = 1 - exp(-den * dt * 14)
                alpha += a * (1 - alpha)
                if alpha > 0.97 { break }
            }
            t += dt
        }
        return alpha
    }

    static func sunOcclusion(size: CGSize, time: Float, drift: SIMD2<Float>, coverage: Float, deck: Float) -> Float {
        guard coverage > 0.001 || deck > 0.001, size.width > 0, size.height > 0 else { return 0 }
        let s = SIMD2(Float(size.width), Float(size.height))
        let aspect = s.x / s.y
        let r = min(s.x, s.y) * 0.12
        let offsets: [SIMD2<Float>] = [.zero, SIMD2(r, 0), SIMD2(-r, 0), SIMD2(0, r), SIMD2(0, -r * 0.3)]
        let total = offsets.reduce(Float(0)) { sum, offset in
            let rd = viewRay(sunUV + offset / s, aspect: aspect)
            return sum + cloudAlpha(along: rd, time: time, drift: drift, coverage: coverage, deck: deck)
        }
        return total / Float(offsets.count)
    }

    private static func mix(_ a: Float, _ b: Float, _ t: Float) -> Float {
        a + (b - a) * t
    }

    private static func smoothstep(_ edge0: Float, _ edge1: Float, _ x: Float) -> Float {
        let t = min(max((x - edge0) / (edge1 - edge0), 0), 1)
        return t * t * (3 - 2 * t)
    }
}
