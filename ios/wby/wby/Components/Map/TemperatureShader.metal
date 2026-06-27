#include <metal_stdlib>
using namespace metal;

// Keep in sync with TemperatureMetalRenderer.swift.
constant uint kMaxSamples = 2048;

struct Uniforms {
    float topMercY;
    float botMercY;
    float leftLon;
    float rightLon;
    uint  sampleCount;
    float coverageInner;
    float coverageOuter;
    float baseAlpha;
    // Grid-path only: geographic bounds (cell centres) and dimensions of the
    // texture field. Ignored by the point-cloud fragment.
    float gridMinLat;
    float gridMaxLat;
    float gridMinLon;
    float gridMaxLon;
    float gridRows;
    float gridCols;
};

struct Sample {
    float2 coord;   // lat, lon
    float  temp;
    float  _padding;
};

struct VertexOut {
    float4 position [[position]];
    float2 uv;
};

vertex VertexOut temperature_vertex(uint vertexID [[vertex_id]]) {
    // Full-screen triangle; covers clip space [-1, 1] x [-1, 1].
    const float2 positions[3] = {
        float2(-1.0, -1.0),
        float2( 3.0, -1.0),
        float2(-1.0,  3.0)
    };
    VertexOut out;
    out.position = float4(positions[vertexID], 0.0, 1.0);
    out.uv = (positions[vertexID] * 0.5 + 0.5);
    return out;
}

constant float3 kPaletteStops[8] = {
    float3( 80.0,  30.0, 130.0) / 255.0,
    float3( 30.0,  55.0, 150.0) / 255.0,
    float3( 55.0, 115.0, 220.0) / 255.0,
    float3( 80.0, 190.0, 180.0) / 255.0,
    float3(180.0, 215.0,  75.0) / 255.0,
    float3(245.0, 210.0,  55.0) / 255.0,
    float3(210.0,  50.0,  40.0) / 255.0,
    float3(109.0,  22.0,  11.0) / 255.0
};

constant float kPaletteTemps[8] = { -40.0, -20.0, -10.0, 0.0, 10.0, 20.0, 30.0, 40.0 };

static float3 rampColor(float temp) {
    if (temp <= kPaletteTemps[0]) { return kPaletteStops[0]; }
    for (int i = 0; i < 7; ++i) {
        float a = kPaletteTemps[i];
        float b = kPaletteTemps[i + 1];
        if (temp <= b) {
            float t = (temp - a) / (b - a);
            return mix(kPaletteStops[i], kPaletteStops[i + 1], t);
        }
    }
    return kPaletteStops[7];
}

static float inverseMercatorLat(float mercY) {
    float latRad = 2.0 * atan(exp(mercY)) - M_PI_F / 2.0;
    return latRad * 180.0 / M_PI_F;
}

fragment float4 temperature_fragment(
    VertexOut in [[stage_in]],
    constant Uniforms& uniforms [[buffer(0)]],
    constant Sample* samples    [[buffer(1)]]
) {
    // UV origin is bottom-left after the positional trick above, but MTKView
    // uses top-left. Flip Y so v=0 maps to the northern (top) edge.
    float v = 1.0 - in.uv.y;
    float u = in.uv.x;

    float mercY = mix(uniforms.topMercY, uniforms.botMercY, v);
    float lat = inverseMercatorLat(mercY);
    float lon = mix(uniforms.leftLon, uniforms.rightLon, u);

    float cosLat = cos(lat * M_PI_F / 180.0);

    // IDW interpolation with scaling to compensate for longitude convergence.
    float sumWeight = 0.0;
    float sumTemp = 0.0;
    float nearestSq = 1e10;
    uint n = min(uniforms.sampleCount, kMaxSamples);
    for (uint i = 0; i < n; ++i) {
        Sample s = samples[i];
        float dLat = lat - s.coord.x;
        float dLon = (lon - s.coord.y) * cosLat;
        float distSq = dLat * dLat + dLon * dLon;
        nearestSq = min(nearestSq, distSq);
        float w = 1.0 / (distSq + 1e-6);
        sumWeight += w;
        sumTemp += w * s.temp;
    }

    if (sumWeight <= 0.0) {
        return float4(0.0);
    }

    float temp = sumTemp / sumWeight;
    float3 rgb = rampColor(temp);

    // Coverage mask (smoothstep between inner/outer distance).
    float dist = sqrt(nearestSq);
    float alpha;
    if (dist <= uniforms.coverageInner) {
        alpha = uniforms.baseAlpha;
    } else if (dist >= uniforms.coverageOuter) {
        alpha = 0.0;
    } else {
        float t = 1.0 - (dist - uniforms.coverageInner) /
                        (uniforms.coverageOuter - uniforms.coverageInner);
        float smooth = t * t * (3.0 - 2.0 * t);
        alpha = smooth * uniforms.baseAlpha;
    }

    if (alpha <= 0.0) {
        return float4(0.0);
    }

    return float4(rgb * alpha, alpha);
}

// Grid path: a field is a regular lat/lon raster uploaded as an rg16Float
// texture where r = value*valid and g = valid. Hardware bilinear gives smooth
// interpolation with no IDW bull's-eyes; dividing the bilinearly-sampled r by g
// (normalized convolution) keeps masked cells from bleeding in.
//
// sampleField returns (value, weight). weight < 0 means the fragment is outside
// the grid; weight == 0 means a fully-masked neighbourhood — both discard.
static float2 sampleField(float2 uv, constant Uniforms& uniforms, texture2d<float> field) {
    constexpr sampler fieldSampler(coord::normalized,
                                   address::clamp_to_edge,
                                   filter::linear);

    float v = 1.0 - uv.y;
    float u = uv.x;

    float mercY = mix(uniforms.topMercY, uniforms.botMercY, v);
    float lat = inverseMercatorLat(mercY);
    float lon = mix(uniforms.leftLon, uniforms.rightLon, u);

    // Position within the grid's cell-centre span, then a half-texel correction
    // so bilinear aligns to cell centres (row 0 / v=0 is the northern edge).
    float pu = (lon - uniforms.gridMinLon) / (uniforms.gridMaxLon - uniforms.gridMinLon);
    float pv = (uniforms.gridMaxLat - lat) / (uniforms.gridMaxLat - uniforms.gridMinLat);
    if (pu < 0.0 || pu > 1.0 || pv < 0.0 || pv > 1.0) {
        return float2(0.0, -1.0);
    }
    float tu = (pu * (uniforms.gridCols - 1.0) + 0.5) / uniforms.gridCols;
    float tv = (pv * (uniforms.gridRows - 1.0) + 0.5) / uniforms.gridRows;

    float2 rg = field.sample(fieldSampler, float2(tu, tv)).rg;
    if (rg.g < 0.02) {
        return float2(0.0, 0.0);
    }
    return float2(rg.r / rg.g, rg.g);
}

fragment float4 temperature_grid_fragment(
    VertexOut in [[stage_in]],
    constant Uniforms& uniforms [[buffer(0)]],
    texture2d<float> field      [[texture(0)]]
) {
    float2 vw = sampleField(in.uv, uniforms, field);
    float weight = vw.y;
    if (weight <= 0.0) {
        return float4(0.0);
    }
    float3 rgb = rampColor(vw.x);
    float alpha = uniforms.baseAlpha * smoothstep(0.0, 0.6, clamp(weight, 0.0, 1.0));
    if (alpha <= 0.0) {
        return float4(0.0);
    }
    return float4(rgb * alpha, alpha);
}

constant float3 kPrecipStops[8] = {
    float3(120.0, 180.0, 235.0) / 255.0,
    float3( 70.0, 110.0, 220.0) / 255.0,
    float3( 60.0, 180.0, 160.0) / 255.0,
    float3( 90.0, 200.0,  90.0) / 255.0,
    float3(235.0, 210.0,  70.0) / 255.0,
    float3(235.0, 150.0,  60.0) / 255.0,
    float3(210.0,  50.0,  40.0) / 255.0,
    float3(180.0,  40.0, 150.0) / 255.0
};

constant float kPrecipRates[8] = { 0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 20.0, 50.0 };

// Below this mm/h a pixel is dry (transparent); alpha ramps from
// kPrecipMinAlphaFrac of base up to full at kPrecipOpaque.
constant float kPrecipDry = 0.1;
constant float kPrecipOpaque = 1.0;
constant float kPrecipMinAlphaFrac = 0.45;

static float3 precipRampColor(float rate) {
    if (rate <= kPrecipRates[0]) { return kPrecipStops[0]; }
    for (int i = 0; i < 7; ++i) {
        float a = kPrecipRates[i];
        float b = kPrecipRates[i + 1];
        if (rate <= b) {
            float t = (rate - a) / (b - a);
            return mix(kPrecipStops[i], kPrecipStops[i + 1], t);
        }
    }
    return kPrecipStops[7];
}

fragment float4 precipitation_grid_fragment(
    VertexOut in [[stage_in]],
    constant Uniforms& uniforms [[buffer(0)]],
    texture2d<float> field      [[texture(0)]]
) {
    float2 vw = sampleField(in.uv, uniforms, field);
    float weight = vw.y;
    float rate = vw.x;
    if (weight <= 0.0 || rate < kPrecipDry) {
        return float4(0.0);
    }
    float3 rgb = precipRampColor(rate);
    float intensity = rate >= kPrecipOpaque
        ? 1.0
        : (rate - kPrecipDry) / (kPrecipOpaque - kPrecipDry);
    float frac = kPrecipMinAlphaFrac + (1.0 - kPrecipMinAlphaFrac) * intensity;
    float alpha = uniforms.baseAlpha * clamp(weight, 0.0, 1.0) * frac;
    if (alpha <= 0.0) {
        return float4(0.0);
    }
    return float4(rgb * alpha, alpha);
}
