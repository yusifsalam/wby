#include <metal_stdlib>
using namespace metal;

// Shaders are consumed by SwiftUI's ShaderLibrary via `Shape.fill(ShaderLibrary.x(...))`.
// Each function takes a `position` in shape-local points and returns a premultiplied half4.

// MARK: - Hash / noise helpers

static float hash21(float2 p) {
    p = fract(p * float2(123.34, 456.21));
    p += dot(p, p + 45.32);
    return fract(p.x * p.y);
}

static float2 hash22(float2 p) {
    return float2(hash21(p), hash21(p + 13.37));
}

static float valueNoise(float2 p) {
    float2 i = floor(p);
    float2 f = fract(p);
    float2 u = f * f * (3.0 - 2.0 * f);
    float a = hash21(i);
    float b = hash21(i + float2(1.0, 0.0));
    float c = hash21(i + float2(0.0, 1.0));
    float d = hash21(i + float2(1.0, 1.0));
    return mix(mix(a, b, u.x), mix(c, d, u.x), u.y);
}

static float fbm(float2 p) {
    float v = 0.0;
    float a = 0.5;
    for (int i = 0; i < 3; ++i) {
        v += a * valueNoise(p);
        p *= 2.03;
        a *= 0.5;
    }
    return v;
}

// MARK: - Stars

[[ stitchable ]] half4 wby_stars(float2 pos, float2 size, float time) {
    // Cell grid roughly 22 × 34 across any screen; tweak for density.
    float2 cellSize = float2(size.x / 22.0, size.y / 34.0);
    float2 p = pos;
    p.x -= time * 3.5; // slow drift to the left

    float2 cell = floor(p / cellSize);
    float rnd = hash21(cell);

    // ~40% of cells carry a star; jittered inside the cell.
    if (rnd < 0.60) { return half4(0.0); }

    float2 cellPos = fract(p / cellSize);
    float2 starPos = float2(0.2 + hash21(cell + 1.7) * 0.6,
                            0.2 + hash21(cell + 3.1) * 0.6);
    float2 d = cellPos - starPos;
    d.x *= cellSize.x / cellSize.y; // compensate for non-square cells
    float dist = length(d);

    float baseRadius = 0.03 + hash21(cell + 9.1) * 0.04;
    float intensity = smoothstep(baseRadius, 0.0, dist);
    if (intensity <= 0.0) { return half4(0.0); }

    // Twinkle.
    float phase = hash21(cell + 5.0) * 6.2831853;
    float freq = 0.35 + hash21(cell + 2.7) * 0.55;
    float twinkle = 0.35 + 0.65 * (0.5 + 0.5 * sin(time * freq + phase));

    // Fade toward the bottom of the screen.
    float y01 = clamp(pos.y / size.y, 0.0, 1.0);
    float skyMask = 1.0 - smoothstep(0.55, 0.90, y01);

    float a = intensity * twinkle * skyMask;
    return half4(half3(1.0) * half(a), half(a));
}

// MARK: - Sun

[[ stitchable ]] half4 wby_sun(float2 pos, float2 size, float time) {
    float shortestSide = max(1.0, min(size.x, size.y));
    float2 center = float2(size.x * 0.20, size.y * 0.04);
    float2 frameCenter = float2(size.x * 0.50, size.y * 0.45);

    float2 delta = pos - center;
    float dist = length(delta);

    float coreRadius = shortestSide * 0.330;
    float haloRadius = shortestSide * 0.800;

    float core = smoothstep(coreRadius, coreRadius * 0.55, dist);
    float halo = smoothstep(haloRadius, coreRadius, dist);
    float y01 = clamp(pos.y / size.y, 0.0, 1.0);
    // Keep the sun strictly in the top band so it never bleeds into scrolled cards.
    float sunMask = 1.0 - smoothstep(0.16, 0.24, y01);
    // Flare can live lower than the sun, but still fades out before lower content.
    float flareMask = 1.0 - smoothstep(0.52, 0.70, y01);

    // Very light pulse so the sun is alive without drawing attention from content.
    float pulse = 0.96 + 0.04 * sin(time * 0.28);
    float alpha = clamp(core * 0.90 + halo * 0.30, 0.0, 1.0) * pulse * sunMask;

    half3 colCore = half3(1.0, 0.96, 0.82);
    half3 colHalo = half3(1.0, 0.89, 0.60);
    half3 sunCol = mix(colHalo, colCore, half(core));

    // Lens flare inspired by camera optics: soft circular ghosts and faint rings.
    float2 axis = frameCenter - center;
    float axisLen = max(1.0, length(axis));
    float2 axisDir = axis / axisLen;
    float2 axisPerp = float2(-axisDir.y, axisDir.x);

    float2 ghost1Center = center + axisDir * shortestSide * 0.42;
    float2 ghost2Center = center + axisDir * shortestSide * 0.80;
    float2 ghost3Center = center + axisDir * shortestSide * 1.12;

    float ghost1 = smoothstep(shortestSide * 0.115, shortestSide * 0.0, length(pos - ghost1Center));
    float ghost2 = smoothstep(shortestSide * 0.088, shortestSide * 0.0, length(pos - ghost2Center));
    float ghost3 = smoothstep(shortestSide * 0.060, shortestSide * 0.0, length(pos - ghost3Center));

    // Irregular lens rings: elliptical and partially broken, not perfect circles.
    float2 rg1 = pos - ghost1Center;
    float2 rg2 = pos - ghost2Center;
    float2 rg1Warp = float2(dot(rg1, axisDir), dot(rg1, axisPerp) * 1.33);
    float2 rg2Warp = float2(dot(rg2, axisDir), dot(rg2, axisPerp) * 0.82);

    float ring1Dist = abs(length(rg1Warp) - shortestSide * 0.155);
    float ring2Dist = abs(length(rg2Warp) - shortestSide * 0.112);
    float ring1 = smoothstep(shortestSide * 0.021, 0.0, ring1Dist);
    float ring2 = smoothstep(shortestSide * 0.016, 0.0, ring2Dist);

    float ring1Arc = 0.56 + 0.44 * sin(atan2(rg1Warp.y, rg1Warp.x) * 3.0 + 0.6);
    float ring2Arc = 0.56 + 0.44 * sin(atan2(rg2Warp.y, rg2Warp.x) * 2.0 - 1.1);
    ring1 *= ring1Arc;
    ring2 *= ring2Arc;

    float broadGlow = smoothstep(shortestSide * 1.05, coreRadius * 0.75, dist);

    float warmFlareAlpha = (broadGlow * 0.050 + ghost1 * 0.110 + ghost2 * 0.082 + ghost3 * 0.060) * flareMask;
    float coolFlareAlpha = (ring1 * 0.060 + ring2 * 0.045) * flareMask;

    half3 flareWarm = half3(1.0, 0.95, 0.84);
    half3 flareCool = half3(0.82, 0.91, 1.0);

    float outA = clamp(alpha + warmFlareAlpha + coolFlareAlpha, 0.0, 1.0);
    if (outA <= 0.001) { return half4(0.0); }

    half3 outRGB = sunCol * half(alpha)
        + flareWarm * half(warmFlareAlpha)
        + flareCool * half(coolFlareAlpha);
    return half4(outRGB, half(outA));
}

// MARK: - Clouds

// Signed-distance for one cumulus silhouette in local coords (scale 1).
// Union of five circles arranged as body + two side lobes + two top bumps.
// `v` is a 0..1 per-cloud variation that nudges the top-bump positions.
static float cloud_sdf(float2 local, float v) {
    float d = length(local) - 0.90;
    d = min(d, length(local - float2(-1.12, -0.08)) - 0.75);
    d = min(d, length(local - float2( 1.05, -0.04)) - 0.78);
    d = min(d, length(local - float2(-0.38 + v * 0.22, 0.58)) - 0.55);
    d = min(d, length(local - float2( 0.42 - v * 0.26, 0.66)) - 0.50);
    return d;
}

[[ stitchable ]] half4 wby_clouds(float2 pos, float2 size, float time, float coverage, float4 tint) {
    // Pixel-scale domain: cloud "unit" ~ 190 points. Works across device sizes.
    float2 p = pos / 190.0;
    p.x -= time * 0.035;

    // Domain warp — samples fbm twice to offset the main lookup, which hides the
    // value-noise grid and yields organic, billowing shapes.
    float2 warp = float2(
        fbm(p + float2(1.7, 9.1)),
        fbm(p + float2(8.3, 2.6))
    );
    float density = fbm(p * 0.75 + warp * 0.95);

    // Coverage drives how much of the sky is cloudy by sliding the threshold.
    // fbm mean ≈ 0.44, std ≈ 0.12. At cov=0, threshold is above nearly all values;
    // at cov=1, threshold is below nearly all values.
    float cov = clamp(coverage, 0.0, 1.0);
    float thresh = mix(0.72, 0.14, cov);

    // Crisp outer edge → defined cloud silhouettes.
    float baseMask = smoothstep(thresh, thresh + 0.08, density);
    // Tighter inner threshold picks out sunlit cores, for outline contrast.
    float litCore = smoothstep(thresh + 0.07, thresh + 0.22, density);

    // Clouds live in the upper ~60% of the view.
    float y01 = clamp(pos.y / size.y, 0.0, 1.0);
    float band = 1.0 - smoothstep(0.58, 0.94, y01);
    baseMask *= band;

    if (baseMask <= 0.001) { return half4(0.0); }

    // Shadow tone and sunlit tone, mixed by the local density core.
    half3 sunlit = half3(tint.rgb);
    half3 shadow = sunlit * half3(0.60);
    half3 rgb = mix(shadow, sunlit, half(litCore));
    half alpha = half(baseMask * tint.a);
    return half4(rgb * alpha, alpha);
}

// MARK: - Rain

static float wby_rain_layer(
    float2 pos,
    float time,
    float colW,
    float speed,
    float density,
    float streakLen,
    float period,
    float halfW,
    float2x2 rot,
    float layerSeed
) {
    float2 p = rot * pos;
    float col = floor(p.x / colW);
    float keep = hash21(float2(col, layerSeed + 41.0));
    if (keep > density) { return 0.0; }

    float colOffset = 0.2 + hash21(float2(col, layerSeed + 17.0)) * 0.6;
    float streakX = (col + colOffset) * colW;
    float xDist = abs(p.x - streakX);
    if (xDist > halfW) { return 0.0; }

    float phase = hash21(float2(col, layerSeed + 7.0)) * 1000.0;
    float y = time * speed - p.y + phase;       // tip advances downward as time grows
    float yMod = fract(y / period) * period;
    if (yMod > streakLen) { return 0.0; }

    float yFall = 1.0 - (yMod / streakLen);    // bright head, fading tail
    float xFall = 1.0 - (xDist / halfW);
    return yFall * xFall;
}

// Maps `intensity` (0..3) to a rain regime.
//   w    is 0..1 spanning drizzle -> heavy rain
//   extra is 0..1 for storm/torrential density boost past w=1
[[ stitchable ]] half4 wby_rain(float2 pos, float2 size, float time, float intensity) {
    float intens = clamp(intensity, 0.0, 3.0);
    float w = clamp(intens / 1.8, 0.0, 1.0);
    float extra = clamp((intens - 1.8) / 1.2, 0.0, 1.0);

    // Streak shape: short/thin/slow/near-vertical at drizzle -> long/wide/fast/tilted at heavy rain.
    float streakLen = mix(9.0, 52.0, w);
    float halfW     = mix(0.45, 0.95, w);
    float period    = mix(110.0, 180.0, w);
    float speed1    = mix(280.0, 740.0, w);
    float speed2    = mix(340.0, 960.0, w);
    float speed3    = mix(220.0, 560.0, w);

    float angle = mix(0.04, 0.16, w);
    float c = cos(angle);
    float s = sin(angle);
    float2x2 rot = float2x2(c, -s, s, c);

    // Per-layer streak-occurrence probability. `w` pushes from sparse to dense;
    // `extra` piles on for storms.
    float d1 = clamp(mix(0.18, 0.85, w) + extra * 0.12, 0.0, 1.0);
    float d2 = clamp(mix(0.12, 0.65, w) + extra * 0.10, 0.0, 1.0);
    float d3 = clamp(mix(0.08, 0.45, w) + extra * 0.08, 0.0, 1.0);

    float a1 = wby_rain_layer(pos, time, 14.0, speed1, d1, streakLen,         period,        halfW,        rot, 1.0);
    float a2 = wby_rain_layer(pos, time, 22.0, speed2, d2, streakLen * 1.05,  period * 1.02, halfW * 1.02, rot, 2.0);
    float a3 = wby_rain_layer(pos, time, 34.0, speed3, d3, streakLen * 0.85,  period * 0.95, halfW * 0.90, rot, 3.0);

    float a = min(a1 * 0.55 + a2 * 0.50 + a3 * 0.38, 1.0);
    half3 col = half3(0.85, 0.90, 0.98);
    // Drizzle is slightly dimmer overall; heavier rain is brighter.
    half alpha = half(a * mix(0.55, 0.78, w));
    return half4(col * alpha, alpha);
}

// MARK: - Snow

static float wby_snow_layer(
    float2 pos,
    float time,
    float cellSize,
    float fallSpeed,
    float density,
    float layerSeed
) {
    float2 p = pos;
    p.y -= time * fallSpeed;  // flakes fall down as time advances

    float2 cell = floor(p / cellSize);
    float keep = hash21(cell + layerSeed * 11.0);
    if (keep > density) { return 0.0; }

    float2 cellUV = fract(p / cellSize);
    float2 center = float2(
        0.25 + hash21(cell + layerSeed + 3.1) * 0.5,
        0.25 + hash21(cell + layerSeed + 5.7) * 0.5
    );
    // Per-flake horizontal wafting.
    float waftPhase = hash21(cell + layerSeed + 9.9) * 6.2831853;
    center.x += sin(time * 0.5 + waftPhase) * 0.14;

    float2 d = cellUV - center;
    float dist = length(d);
    float radius = 0.06 + hash21(cell + layerSeed + 7.7) * 0.08;
    float a = smoothstep(radius, 0.0, dist);
    return a * (0.55 + hash21(cell + layerSeed + 2.2) * 0.45);
}

[[ stitchable ]] half4 wby_snow(float2 pos, float2 size, float time, float intensity) {
    float intens = clamp(intensity, 0.3, 3.0);

    float d1 = clamp(intens * 0.55, 0.0, 1.0);
    float d2 = clamp(intens * 0.45, 0.0, 1.0);
    float d3 = clamp(intens * 0.38, 0.0, 1.0);

    float a1 = wby_snow_layer(pos, time, 22.0, 110.0, d1, 1.0);
    float a2 = wby_snow_layer(pos, time, 32.0, 150.0, d2, 2.0);
    float a3 = wby_snow_layer(pos, time, 46.0, 190.0, d3, 3.0);

    float a = min(a1 * 0.75 + a2 * 0.65 + a3 * 0.55, 1.0);
    half alpha = half(a * 0.85);
    return half4(half3(1.0) * alpha, alpha);
}
