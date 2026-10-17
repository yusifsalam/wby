#include <metal_stdlib>
using namespace metal;

// Shaders are consumed by SwiftUI's ShaderLibrary via `Shape.fill(ShaderLibrary.x(...))`.
// Each function takes a `position` in shape-local points and returns a premultiplied half4.
// The cloud volume is mirrored in SkyNoise.swift.

// MARK: - Hash / noise helpers

static uint wby_hash_bits(int2 p) {
    uint2 x = uint2(p);
    uint2 q = 1103515245u * ((x >> 1u) ^ x.yx);
    return 1103515245u * (q.x ^ (q.y >> 3u));
}

static float wby_hash(int2 p) {
    return float(wby_hash_bits(p)) * (1.0 / 4294967295.0);
}

static float wby_noise(float2 p) {
    float2 i = floor(p);
    float2 f = p - i;
    float2 u = f * f * (3.0 - 2.0 * f);
    int2 c = int2(i);
    float a = wby_hash(c);
    float b = wby_hash(c + int2(1, 0));
    float d = wby_hash(c + int2(0, 1));
    float e = wby_hash(c + int2(1, 1));
    return mix(mix(a, b, u.x), mix(d, e, u.x), u.y);
}

static float wby_hash3(int3 p) {
    uint3 x = uint3(p);
    uint n = (x.x * 1597334673u) ^ (x.y * 3812015801u) ^ (x.z * 2798796415u);
    n = (n ^ (n >> 15u)) * 2246822519u;
    n = (n ^ (n >> 13u)) * 3266489917u;
    return float(n ^ (n >> 16u)) * (1.0 / 4294967295.0);
}

static float wby_noise3(float3 p) {
    float3 i = floor(p);
    float3 f = p - i;
    float3 u = f * f * (3.0 - 2.0 * f);
    int3 c = int3(i);
    float x00 = mix(wby_hash3(c), wby_hash3(c + int3(1, 0, 0)), u.x);
    float x10 = mix(wby_hash3(c + int3(0, 1, 0)), wby_hash3(c + int3(1, 1, 0)), u.x);
    float x01 = mix(wby_hash3(c + int3(0, 0, 1)), wby_hash3(c + int3(1, 0, 1)), u.x);
    float x11 = mix(wby_hash3(c + int3(0, 1, 1)), wby_hash3(c + int3(1, 1, 1)), u.x);
    return mix(mix(x00, x10, u.y), mix(x01, x11, u.y), u.z);
}

static float3 wby_rotate3(float3 p) {
    return float3(
        -0.80 * p.y - 0.60 * p.z,
        0.80 * p.x + 0.36 * p.y - 0.48 * p.z,
        0.60 * p.x - 0.48 * p.y + 0.64 * p.z
    );
}

static float4 wby_over(float4 top, float4 bottom) {
    return top + bottom * (1.0 - top.a);
}

// MARK: - Stars

[[ stitchable ]] half4 wby_stars(float2 pos, float2 size, float time, float amount) {
    float y01 = pos.y / size.y;
    if (amount <= 0.001 || y01 > 0.9) { return half4(0.0); }

    float2 cellSize = float2(18.0, 25.0);
    float2 p = float2(pos.x - time * 3.5, pos.y);
    float2 cellF = floor(p / cellSize);
    int2 cell = int2(cellF);
    if (wby_hash(cell) < 0.60) { return half4(0.0); }

    float2 cellPos = p / cellSize - cellF;
    float2 starPos = float2(0.2 + wby_hash(cell + int2(17, 3)) * 0.6,
                            0.2 + wby_hash(cell + int2(31, 7)) * 0.6);
    float2 d = cellPos - starPos;
    d.x *= cellSize.x / cellSize.y;
    float radius = 0.03 + wby_hash(cell + int2(91, 11)) * 0.04;
    float intensity = smoothstep(radius, 0.0, length(d));
    if (intensity <= 0.0) { return half4(0.0); }

    float phase = wby_hash(cell + int2(5, 50)) * 6.2831853;
    float freq = 0.35 + wby_hash(cell + int2(27, 2)) * 0.55;
    float twinkle = 0.35 + 0.65 * (0.5 + 0.5 * sin(time * freq + phase));
    float3 tint = mix(float3(0.80, 0.87, 1.0), float3(1.0, 0.93, 0.82), wby_hash(cell + int2(63, 19)));
    float a = intensity * twinkle * amount * (1.0 - smoothstep(0.55, 0.90, y01));
    return half4(half3(tint * a), half(a));
}

// MARK: - Cloud volume

constant float wby_pitch = 0.50;
constant float wby_tan_half_fov = 0.80;
constant float wby_layer_bottom = 1.0;
constant float2 wby_sun_uv = float2(0.20, 0.04);

static float3 wby_view_ray(float2 uv, float aspect) {
    float2 ndc = float2((uv.x - 0.5) * 2.0 * aspect, (0.5 - uv.y) * 2.0) * wby_tan_half_fov;
    float s = sin(wby_pitch);
    float c = cos(wby_pitch);
    return normalize(float3(ndc.x, s + c * ndc.y, c - s * ndc.y));
}

static float wby_layer_top(float deck) {
    return wby_layer_bottom + mix(0.45, 0.65, deck);
}

static float wby_cloud_density(float3 p, float coverage, float deck, float top, float3 offset, int octaves) {
    float h = (p.y - wby_layer_bottom) / (top - wby_layer_bottom);
    float cumulusProfile = smoothstep(0.0, 0.12, h) * (1.0 - smoothstep(0.25, 1.0, h));
    float deckProfile = smoothstep(0.0, 0.10, h) * (1.0 - smoothstep(0.55, 1.0, h));
    float profile = mix(cumulusProfile, deckProfile, deck);
    float threshold = mix(0.78, 0.30, coverage);
    if (profile <= threshold) { return 0.0; }

    float weight = 1.0 - 1.0 / float(1 << octaves);
    float needed = threshold * weight / profile;
    float3 q = p * mix(3.0, 3.8, deck) + offset;
    float f = 0.5 * wby_noise3(q);
    if (f + weight - 0.5 <= needed) { return 0.0; }
    q = wby_rotate3(q) * 2.02;
    f += 0.25 * wby_noise3(q);
    if (f + weight - 0.75 <= needed) { return 0.0; }
    if (octaves > 2) {
        q = wby_rotate3(q) * 2.03;
        f += 0.125 * wby_noise3(q);
        if (f + weight - 0.875 <= needed) { return 0.0; }
    }
    if (octaves > 3) {
        q = wby_rotate3(q) * 2.01;
        f += 0.0625 * wby_noise3(q);
        if (f + weight - 0.9375 <= needed) { return 0.0; }
    }
    if (octaves > 4) {
        q = wby_rotate3(q) * 2.04;
        f += 0.03125 * wby_noise3(q);
    }
    f /= weight;

    return clamp((f * profile - threshold) / (1.0 - threshold) * 3.2, 0.0, 1.0);
}

static float4 wby_cloud_volume(
    float3 rd,
    float3 sunDir,
    float time,
    float2 drift,
    float4 clouds,
    float sun,
    float night,
    float3 lit,
    float3 shadow,
    float4 flash,
    float aspect,
    float jitter
) {
    if (rd.y < 0.03) { return float4(0.0); }
    float coverage = clouds.x;
    float deck = clouds.y;
    float absorption = clouds.w;
    float top = wby_layer_top(deck);
    float t0 = wby_layer_bottom / rd.y;
    if (t0 > 9.0) { return float4(0.0); }
    float t1 = min(top / rd.y, 9.0);

    const int steps = 18;
    float dt = (t1 - t0) / float(steps);
    float t = t0 + dt * jitter;
    float3 offset = float3(-drift.x, time * 0.003, -drift.y);

    float cosSun = dot(rd, sunDir);
    float forward = pow(clamp(cosSun, 0.0, 1.0), 6.0);
    float phase = 0.8 + sun * forward * 1.1;

    float3 flashPos = float3(0.0);
    if (flash.x > 0.001) {
        float3 fr = wby_view_ray(float2(flash.y, 0.22), aspect);
        flashPos = fr * ((wby_layer_bottom + 0.25) / max(fr.y, 0.05));
    }

    float4 sum = float4(0.0);
    for (int i = 0; i < steps; ++i) {
        float3 p = rd * t;
        float den = wby_cloud_density(p, coverage, deck, top, offset, t < 1.8 ? 5 : (t < 3.0 ? 4 : 3));
        if (den > 0.003) {
            float denL = wby_cloud_density(p + sunDir * 0.07, coverage, deck, top, offset, 2)
                + wby_cloud_density(p + sunDir * 0.22, coverage, deck, top, offset, 2);
            float h = (p.y - wby_layer_bottom) / (top - wby_layer_bottom);
            float sunT = exp(-denL * absorption * 3.0);
            float powder = 1.0 - exp(-den * 5.0);
            float3 ambient = shadow * mix(0.62, 1.05, h) + lit * night * 0.15 * (1.0 - h);
            float3 color = ambient + lit * sunT * phase * mix(0.6, 1.0, powder) * (1.0 - night * 0.6);
            if (flash.x > 0.001) {
                color += float3(0.80, 0.82, 1.0) * flash.x * 0.9 * exp(-length(p - flashPos) * 3.0) * (0.4 + 0.6 * den);
            }
            float a = 1.0 - exp(-den * dt * 14.0);
            sum.rgb += color * a * (1.0 - sum.a);
            sum.a += a * (1.0 - sum.a);
            if (sum.a > 0.97) { break; }
        }
        t += dt;
    }
    float fade = mix(exp(-max(t0 - 1.6, 0.0) * 0.22), 1.0, deck * 0.7);
    float3 mapped = 1.0 - exp(-sum.rgb * 2.0 / max(sum.a, 0.001));
    return float4(mapped * sum.a, sum.a) * fade;
}

// MARK: - Sky (sun, cirrus, cloud volume, flare)

// clouds: cumulus coverage, deck amount, cirrus amount, absorption
// light: sun amount, sun fade, night, sun occlusion
// flash: lightning intensity, lightning x (0-1)
[[ stitchable ]] half4 wby_sky(
    float2 pos,
    float2 size,
    float2 pixels,
    float time,
    float2 drift,
    float4 clouds,
    float4 light,
    float4 litTint,
    float4 shadowTint,
    float4 flash
) {
    float2 uv = pos / size;
    if (uv.y > 0.9) { return half4(0.0); }
    float aspect = size.x / size.y;
    float shortestSide = max(1.0, min(size.x, size.y));
    float2 sunCenter = wby_sun_uv * size;
    float sunAmount = light.x * light.y;
    float occlusion = clamp(light.w, 0.0, 1.0);
    float3 rd = wby_view_ray(uv, aspect);
    float3 sunDir = wby_view_ray(wby_sun_uv, aspect);

    float4 color = float4(0.0);

    if (sunAmount > 0.001 && uv.y < 0.30) {
        float dist = length(pos - sunCenter);
        float coreRadius = shortestSide * 0.33;
        float core = smoothstep(coreRadius, coreRadius * 0.55, dist);
        float halo = smoothstep(shortestSide * 0.8, coreRadius, dist);
        float sunMask = 1.0 - smoothstep(0.10, 0.30, uv.y);
        float pulse = 0.96 + 0.04 * sin(time * 0.28);
        float a = clamp(core * 0.90 + halo * 0.30, 0.0, 1.0) * pulse * sunMask * sunAmount;
        float3 rgb = mix(float3(1.0, 0.89, 0.60), float3(1.0, 0.96, 0.82), core);
        color = float4(rgb * a, a);
    }

    if (clouds.z > 0.001 && rd.y > 0.08) {
        float2 base = (rd.xz * (3.0 / rd.y)) * 0.55 - drift * 0.4 + float2(17.3, 41.9);
        base = float2(base.x * 0.94 - base.y * 0.34, base.x * 0.34 + base.y * 0.94);
        float patch = smoothstep(0.42, 0.72, wby_noise(base * 0.45 + float2(11.7, 5.2)));
        if (patch > 0.001) {
            float bend = wby_noise(base * 0.8 + float2(3.1, 8.4));
            float2 nc = float2(base.x, base.y * 3.2 + bend * 2.4);
            float c = 0.55 * wby_noise(nc);
            c += 0.30 * wby_noise(nc * float2(2.0, 3.0) + float2(3.3, 7.1));
            c += 0.15 * wby_noise(nc * float2(3.9, 8.5) + float2(9.4, 1.6));
            float a = smoothstep(0.42, 0.82, c) * patch * clouds.z * 0.6 * smoothstep(0.25, 0.55, rd.y);
            color = wby_over(float4(mix(shadowTint.rgb, litTint.rgb, 0.85) * a, a), color);
        }
    }

    if (clouds.x > 0.001 || clouds.y > 0.001) {
        float ign = fract(52.9829189 * fract(dot(floor(uv * pixels), float2(0.06711056, 0.00583715))));
        float jitter = 0.5 + (ign - 0.5) * 0.7 * smoothstep(0.20, 0.55, uv.y);
        float4 cloud = wby_cloud_volume(rd, sunDir, time, drift, clouds, light.x, light.z,
                                        litTint.rgb, shadowTint.rgb, flash, aspect, jitter);
        color = wby_over(cloud * (1.0 - smoothstep(0.52, 0.80, uv.y)), color);
    }

    if (sunAmount > 0.001 && uv.y < 0.72) {
        float2 frameCenter = float2(size.x * 0.50, size.y * 0.45);
        float2 axis = frameCenter - sunCenter;
        float2 axisDir = axis / max(1.0, length(axis));
        float2 axisPerp = float2(-axisDir.y, axisDir.x);

        float2 ghost1Center = sunCenter + axisDir * shortestSide * 0.42;
        float2 ghost2Center = sunCenter + axisDir * shortestSide * 0.80;
        float2 ghost3Center = sunCenter + axisDir * shortestSide * 1.12;

        float ghost1 = smoothstep(shortestSide * 0.115, 0.0, length(pos - ghost1Center));
        float ghost2 = smoothstep(shortestSide * 0.088, 0.0, length(pos - ghost2Center));
        float ghost3 = smoothstep(shortestSide * 0.060, 0.0, length(pos - ghost3Center));

        float2 rg1 = pos - ghost1Center;
        float2 rg2 = pos - ghost2Center;
        float2 rg1Warp = float2(dot(rg1, axisDir), dot(rg1, axisPerp) * 1.33);
        float2 rg2Warp = float2(dot(rg2, axisDir), dot(rg2, axisPerp) * 0.82);
        float ring1 = smoothstep(shortestSide * 0.021, 0.0, abs(length(rg1Warp) - shortestSide * 0.155));
        float ring2 = smoothstep(shortestSide * 0.016, 0.0, abs(length(rg2Warp) - shortestSide * 0.112));
        ring1 *= 0.56 + 0.44 * sin(atan2(rg1Warp.y, rg1Warp.x) * 3.0 + 0.6);
        ring2 *= 0.56 + 0.44 * sin(atan2(rg2Warp.y, rg2Warp.x) * 2.0 - 1.1);

        float broadGlow = smoothstep(shortestSide * 1.05, shortestSide * 0.25, length(pos - sunCenter));
        float flareMask = (1.0 - smoothstep(0.52, 0.70, uv.y)) * sunAmount * (1.0 - occlusion);
        float warm = (broadGlow * 0.050 * (1.0 - color.a) + ghost1 * 0.110 + ghost2 * 0.082 + ghost3 * 0.060) * flareMask;
        float cool = (ring1 * 0.060 + ring2 * 0.045) * flareMask;
        float3 rgb = float3(1.0, 0.95, 0.84) * warm + float3(0.82, 0.91, 1.0) * cool;
        color = float4(color.rgb + rgb, min(1.0, color.a + warm + cool));
    }

    if (color.a <= 0.001) { return half4(0.0); }
    return half4(color);
}

// MARK: - Rain

static float wby_rain_layer(
    float2 p,
    float phase,
    float colW,
    float density,
    float streakLen,
    float period,
    float halfW,
    int seed
) {
    float colF = floor(p.x / colW);
    int2 col = int2(int(colF), seed);
    if (wby_hash(col + int2(0, 41)) > density) { return 0.0; }

    float colOffset = 0.2 + wby_hash(col + int2(0, 17)) * 0.6;
    float xDist = abs(p.x - (colF + colOffset) * colW);
    if (xDist > halfW) { return 0.0; }

    float len = streakLen * (0.7 + 0.6 * wby_hash(col + int2(0, 29)));
    float y = phase - p.y + wby_hash(col + int2(0, 7)) * 1000.0;
    float yMod = fract(y / period) * period;
    if (yMod > len) { return 0.0; }

    float yFall = 1.0 - yMod / len;
    float xFall = 1.0 - xDist / halfW;
    return yFall * xFall * (0.6 + 0.4 * wby_hash(col + int2(0, 3)));
}

// intensity: 0-3 (drizzle → torrential); phase: accumulated fall distance in points; slant: radians
[[ stitchable ]] half4 wby_rain(float2 pos, float2 size, float phase, float intensity, float slant) {
    float intens = clamp(intensity, 0.0, 3.0);
    float w = clamp(intens / 1.8, 0.0, 1.0);
    float extra = clamp((intens - 1.8) / 1.2, 0.0, 1.0);

    float streakLen = mix(9.0, 52.0, w);
    float period = mix(110.0, 180.0, w);
    float c = cos(slant);
    float s = sin(slant);
    float2 p = float2(pos.x * c - pos.y * s, pos.x * s + pos.y * c);

    float dFar = clamp(mix(0.20, 0.80, w) + extra * 0.12, 0.0, 1.0);
    float dMid = clamp(mix(0.16, 0.70, w) + extra * 0.10, 0.0, 1.0);
    float dNear = clamp(mix(0.06, 0.38, w) + extra * 0.10, 0.0, 1.0);

    float far = wby_rain_layer(p, phase * 0.55, 9.0, dFar, streakLen * 0.55, period * 0.55, mix(0.30, 0.45, w), 1);
    float mid = wby_rain_layer(p, phase * 0.85, 16.0, dMid, streakLen * 0.85, period * 0.85, mix(0.45, 0.80, w), 2);
    float near = wby_rain_layer(p, phase * 1.25, 30.0, dNear, streakLen * 1.25, period * 1.25, mix(0.80, 1.50, w), 3);

    float3 farCol = float3(0.72, 0.80, 0.94);
    float3 nearCol = float3(0.88, 0.92, 0.99);
    float brightness = mix(0.55, 0.80, w);
    float aFar = far * 0.30 * brightness;
    float aMid = mid * 0.50 * brightness;
    float aNear = sqrt(near) * near * 0.55 * brightness;

    float4 color = float4(farCol * aFar, aFar);
    color = wby_over(float4(mix(farCol, nearCol, 0.5) * aMid, aMid), color);
    color = wby_over(float4(nearCol * aNear, aNear), color);
    if (color.a <= 0.001) { return half4(0.0); }
    return half4(color);
}

// MARK: - Snow

static float wby_snow_layer(
    float2 pos,
    float2 fall,
    float time,
    float cellSize,
    float speed,
    float density,
    float radius,
    float softness,
    float sway,
    int seed
) {
    float2 p = float2(pos.x - fall.x * speed, pos.y);
    float colF = floor(p.x / cellSize);
    float colSpeed = 0.8 + 0.4 * wby_hash(int2(int(colF), seed * 131 + 7));
    p.y -= fall.y * speed * colSpeed;

    float2 cellF = floor(p / cellSize);
    int2 cell = int2(cellF) + int2(0, seed * 7919);
    if (wby_hash(cell + int2(13, 0)) > density) { return 0.0; }

    float2 uv = p / cellSize - cellF;
    float r = radius * (0.6 + 0.4 * wby_hash(cell + int2(7, 77)));
    float amp = sway * (0.5 + 0.5 * wby_hash(cell + int2(9, 99)));
    float margin = r + amp;
    float2 center = float2(mix(margin, 1.0 - margin, wby_hash(cell + int2(3, 31))),
                           mix(r, 1.0 - r, wby_hash(cell + int2(5, 57))));
    float freq = 0.5 + 0.9 * wby_hash(cell + int2(11, 13));
    float phase = wby_hash(cell + int2(23, 29)) * 6.2831853;
    center.x += sin(time * freq + phase) * amp;

    float dist = length(uv - center);
    float a = 1.0 - smoothstep(r * (1.0 - softness), r, dist);
    return a * (0.55 + 0.45 * wby_hash(cell + int2(2, 22)));
}

// fall: accumulated (wind drift, fall distance) in points; wind: east component in m/s
[[ stitchable ]] half4 wby_snow(float2 pos, float2 size, float time, float2 fall, float intensity, float wind) {
    float intens = clamp(intensity, 0.3, 3.0);
    float gust = 1.0 + min(abs(wind), 10.0) * 0.05;

    float far = wby_snow_layer(pos, fall, time, 14.0, 0.55, clamp(intens * 0.50, 0.0, 1.0), 0.11, 0.45, 0.10 * gust, 1);
    float mid = wby_snow_layer(pos, fall, time, 30.0, 1.00, clamp(intens * 0.45, 0.0, 1.0), 0.10, 0.60, 0.12 * gust, 2);
    float near = wby_snow_layer(pos, fall, time, 90.0, 1.70, clamp(intens * 0.16, 0.0, 1.0), 0.085, 1.0, 0.08 * gust, 3);

    float a = min(far * 0.50 + mid * 0.80 + near * 0.50, 1.0) * 0.9;
    if (a <= 0.001) { return half4(0.0); }
    return half4(half3(1.0) * half(a), half(a));
}
