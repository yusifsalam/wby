uniform vec2 u_size;
uniform float u_time;
uniform vec2 u_drift;
uniform vec4 u_clouds;
uniform vec4 u_light;
uniform vec3 u_lit;
uniform vec3 u_shadow;
uniform vec2 u_flash;

out vec4 fragColor;

const float PITCH = 0.50;
const float TAN_HALF_FOV = 0.80;
const float LAYER_BOTTOM = 1.0;
const vec2 SUN_UV = vec2(0.20, 0.04);

float noise(vec2 p) {
  vec2 i = floor(p);
  vec2 f = p - i;
  vec2 u = f * f * (3.0 - 2.0 * f);
  ivec2 c = ivec2(i);
  float a = hash(c);
  float b = hash(c + ivec2(1, 0));
  float d = hash(c + ivec2(0, 1));
  float e = hash(c + ivec2(1, 1));
  return mix(mix(a, b, u.x), mix(d, e, u.x), u.y);
}

float hash3(ivec3 p) {
  uvec3 x = uvec3(p);
  uint n = (x.x * 1597334673u) ^ (x.y * 3812015801u) ^ (x.z * 2798796415u);
  n = (n ^ (n >> 15u)) * 2246822519u;
  n = (n ^ (n >> 13u)) * 3266489917u;
  return float(n ^ (n >> 16u)) * (1.0 / 4294967295.0);
}

float noise3(vec3 p) {
  vec3 i = floor(p);
  vec3 f = p - i;
  vec3 u = f * f * (3.0 - 2.0 * f);
  ivec3 c = ivec3(i);
  float x00 = mix(hash3(c), hash3(c + ivec3(1, 0, 0)), u.x);
  float x10 = mix(hash3(c + ivec3(0, 1, 0)), hash3(c + ivec3(1, 1, 0)), u.x);
  float x01 = mix(hash3(c + ivec3(0, 0, 1)), hash3(c + ivec3(1, 0, 1)), u.x);
  float x11 = mix(hash3(c + ivec3(0, 1, 1)), hash3(c + ivec3(1, 1, 1)), u.x);
  return mix(mix(x00, x10, u.y), mix(x01, x11, u.y), u.z);
}

vec3 rotate3(vec3 p) {
  return vec3(
    -0.80 * p.y - 0.60 * p.z,
    0.80 * p.x + 0.36 * p.y - 0.48 * p.z,
    0.60 * p.x - 0.48 * p.y + 0.64 * p.z
  );
}

vec3 viewRay(vec2 uv, float aspect) {
  vec2 ndc = vec2((uv.x - 0.5) * 2.0 * aspect, (0.5 - uv.y) * 2.0) * TAN_HALF_FOV;
  float s = sin(PITCH);
  float c = cos(PITCH);
  return normalize(vec3(ndc.x, s + c * ndc.y, c - s * ndc.y));
}

float layerTop(float deck) {
  return LAYER_BOTTOM + mix(0.45, 0.65, deck);
}

float cloudDensity(vec3 p, float coverage, float deck, float top, vec3 offset, int octaves) {
  float h = (p.y - LAYER_BOTTOM) / (top - LAYER_BOTTOM);
  float cumulusProfile = ss(0.0, 0.12, h) * (1.0 - ss(0.25, 1.0, h));
  float deckProfile = ss(0.0, 0.10, h) * (1.0 - ss(0.55, 1.0, h));
  float profile = mix(cumulusProfile, deckProfile, deck);
  if (profile <= 0.0) { return 0.0; }

  vec3 q = p * mix(3.0, 3.8, deck) + offset;
  float f = 0.5 * noise3(q);
  q = rotate3(q) * 2.02;
  f += 0.25 * noise3(q);
  float weight = 0.75;
  if (octaves > 2) {
    q = rotate3(q) * 2.03;
    f += 0.125 * noise3(q);
    weight += 0.125;
  }
  if (octaves > 3) {
    q = rotate3(q) * 2.01;
    f += 0.0625 * noise3(q);
    weight += 0.0625;
  }
  if (octaves > 4) {
    q = rotate3(q) * 2.04;
    f += 0.03125 * noise3(q);
    weight += 0.03125;
  }
  f /= weight;

  float threshold = mix(0.78, 0.30, coverage);
  return clamp((f * profile - threshold) / (1.0 - threshold) * 3.2, 0.0, 1.0);
}

vec4 cloudVolume(vec3 rd, vec3 sunDir, float aspect, float jitter) {
  if (rd.y < 0.03) { return vec4(0.0); }
  float coverage = u_clouds.x;
  float deck = u_clouds.y;
  float absorption = u_clouds.w;
  float sun = u_light.x;
  float night = u_light.z;
  float top = layerTop(deck);
  float t0 = LAYER_BOTTOM / rd.y;
  if (t0 > 9.0) { return vec4(0.0); }
  float t1 = min(top / rd.y, 9.0);

  const int steps = 18;
  float dt = (t1 - t0) / float(steps);
  float t = t0 + dt * jitter;
  vec3 offset = vec3(-u_drift.x, u_time * 0.003, -u_drift.y);

  float cosSun = dot(rd, sunDir);
  float forward = pow(clamp(cosSun, 0.0, 1.0), 6.0);
  float phase = 0.8 + sun * forward * 1.1;

  vec3 flashPos = vec3(0.0);
  if (u_flash.x > 0.001) {
    vec3 fr = viewRay(vec2(u_flash.y, 0.22), aspect);
    flashPos = fr * ((LAYER_BOTTOM + 0.25) / max(fr.y, 0.05));
  }

  vec4 sum = vec4(0.0);
  for (int i = 0; i < steps; ++i) {
    vec3 p = rd * t;
    float den = cloudDensity(p, coverage, deck, top, offset, t < 1.8 ? 5 : (t < 3.0 ? 4 : 3));
    if (den > 0.003) {
      float denL = cloudDensity(p + sunDir * 0.07, coverage, deck, top, offset, 2)
        + cloudDensity(p + sunDir * 0.22, coverage, deck, top, offset, 2);
      float h = (p.y - LAYER_BOTTOM) / (top - LAYER_BOTTOM);
      float sunT = exp(-denL * absorption * 3.0);
      float powder = 1.0 - exp(-den * 5.0);
      vec3 ambient = u_shadow * mix(0.62, 1.05, h) + u_lit * night * 0.15 * (1.0 - h);
      vec3 color = ambient + u_lit * sunT * phase * mix(0.6, 1.0, powder) * (1.0 - night * 0.6);
      if (u_flash.x > 0.001) {
        color += vec3(0.80, 0.82, 1.0) * u_flash.x * 0.9 * exp(-length(p - flashPos) * 3.0) * (0.4 + 0.6 * den);
      }
      float a = 1.0 - exp(-den * dt * 14.0);
      sum.rgb += color * a * (1.0 - sum.a);
      sum.a += a * (1.0 - sum.a);
      if (sum.a > 0.97) { break; }
    }
    t += dt;
  }
  float fade = mix(exp(-max(t0 - 1.6, 0.0) * 0.22), 1.0, deck * 0.7);
  vec3 mapped = 1.0 - exp(-sum.rgb * 2.0 / max(sum.a, 0.001));
  return vec4(mapped * sum.a, sum.a) * fade;
}

void main() {
  vec2 pos = vec2(gl_FragCoord.x, u_size.y - gl_FragCoord.y);
  vec2 uv = pos / u_size;
  if (uv.y > 0.9) {
    fragColor = vec4(0.0);
    return;
  }
  float aspect = u_size.x / u_size.y;
  float shortestSide = max(1.0, min(u_size.x, u_size.y));
  vec2 sunCenter = SUN_UV * u_size;
  float sunAmount = u_light.x * u_light.y;
  float occlusion = clamp(u_light.w, 0.0, 1.0);
  vec3 rd = viewRay(uv, aspect);
  vec3 sunDir = viewRay(SUN_UV, aspect);

  vec4 color = vec4(0.0);

  if (sunAmount > 0.001 && uv.y < 0.30) {
    float dist = length(pos - sunCenter);
    float coreRadius = shortestSide * 0.33;
    float core = ss(coreRadius, coreRadius * 0.55, dist);
    float halo = ss(shortestSide * 0.8, coreRadius, dist);
    float sunMask = 1.0 - ss(0.10, 0.30, uv.y);
    float pulse = 0.96 + 0.04 * sin(u_time * 0.28);
    float a = clamp(core * 0.90 + halo * 0.30, 0.0, 1.0) * pulse * sunMask * sunAmount;
    vec3 rgb = mix(vec3(1.0, 0.89, 0.60), vec3(1.0, 0.96, 0.82), core);
    color = vec4(rgb * a, a);
  }

  if (u_clouds.z > 0.001 && rd.y > 0.08) {
    vec2 base = (rd.xz * (3.0 / rd.y)) * 0.55 - u_drift * 0.4 + vec2(17.3, 41.9);
    base = vec2(base.x * 0.94 - base.y * 0.34, base.x * 0.34 + base.y * 0.94);
    float patchMask = ss(0.42, 0.72, noise(base * 0.45 + vec2(11.7, 5.2)));
    if (patchMask > 0.001) {
      float bend = noise(base * 0.8 + vec2(3.1, 8.4));
      vec2 nc = vec2(base.x, base.y * 3.2 + bend * 2.4);
      float c = 0.55 * noise(nc);
      c += 0.30 * noise(nc * vec2(2.0, 3.0) + vec2(3.3, 7.1));
      c += 0.15 * noise(nc * vec2(3.9, 8.5) + vec2(9.4, 1.6));
      float a = ss(0.42, 0.82, c) * patchMask * u_clouds.z * 0.6 * ss(0.25, 0.55, rd.y);
      color = over(vec4(mix(u_shadow, u_lit, 0.85) * a, a), color);
    }
  }

  if (u_clouds.x > 0.001 || u_clouds.y > 0.001) {
    float ign = fract(52.9829189 * fract(dot(floor(pos), vec2(0.06711056, 0.00583715))));
    float jitter = 0.5 + (ign - 0.5) * 0.7 * ss(0.20, 0.55, uv.y);
    vec4 cloud = cloudVolume(rd, sunDir, aspect, jitter);
    color = over(cloud * (1.0 - ss(0.52, 0.80, uv.y)), color);
  }

  if (sunAmount > 0.001 && uv.y < 0.72) {
    vec2 frameCenter = vec2(u_size.x * 0.50, u_size.y * 0.45);
    vec2 axis = frameCenter - sunCenter;
    vec2 axisDir = axis / max(1.0, length(axis));
    vec2 axisPerp = vec2(-axisDir.y, axisDir.x);

    vec2 ghost1Center = sunCenter + axisDir * shortestSide * 0.42;
    vec2 ghost2Center = sunCenter + axisDir * shortestSide * 0.80;
    vec2 ghost3Center = sunCenter + axisDir * shortestSide * 1.12;

    float ghost1 = ss(shortestSide * 0.115, 0.0, length(pos - ghost1Center));
    float ghost2 = ss(shortestSide * 0.088, 0.0, length(pos - ghost2Center));
    float ghost3 = ss(shortestSide * 0.060, 0.0, length(pos - ghost3Center));

    vec2 rg1 = pos - ghost1Center;
    vec2 rg2 = pos - ghost2Center;
    vec2 rg1Warp = vec2(dot(rg1, axisDir), dot(rg1, axisPerp) * 1.33);
    vec2 rg2Warp = vec2(dot(rg2, axisDir), dot(rg2, axisPerp) * 0.82);
    float ring1 = ss(shortestSide * 0.021, 0.0, abs(length(rg1Warp) - shortestSide * 0.155));
    float ring2 = ss(shortestSide * 0.016, 0.0, abs(length(rg2Warp) - shortestSide * 0.112));
    ring1 *= 0.56 + 0.44 * sin(atan(rg1Warp.y, rg1Warp.x) * 3.0 + 0.6);
    ring2 *= 0.56 + 0.44 * sin(atan(rg2Warp.y, rg2Warp.x) * 2.0 - 1.1);

    float broadGlow = ss(shortestSide * 1.05, shortestSide * 0.25, length(pos - sunCenter));
    float flareMask = (1.0 - ss(0.52, 0.70, uv.y)) * sunAmount * (1.0 - occlusion);
    float warm = (broadGlow * 0.050 * (1.0 - color.a) + ghost1 * 0.110 + ghost2 * 0.082 + ghost3 * 0.060) * flareMask;
    float cool = (ring1 * 0.060 + ring2 * 0.045) * flareMask;
    vec3 rgb = vec3(1.0, 0.95, 0.84) * warm + vec3(0.82, 0.91, 1.0) * cool;
    color = vec4(color.rgb + rgb, min(1.0, color.a + warm + cool));
  }

  fragColor = color.a <= 0.001 ? vec4(0.0) : color;
}
