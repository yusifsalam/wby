uniform sampler2D u_sky;
uniform vec2 u_canvas;
uniform float u_scale;
uniform vec2 u_size;
uniform float u_time;
uniform float u_stars;
uniform int u_precipitation;
uniform vec3 u_rain;
uniform vec4 u_snow;
uniform float u_flash;

out vec4 fragColor;

vec4 stars(vec2 pos) {
  float y01 = pos.y / u_size.y;
  if (u_stars <= 0.001 || y01 > 0.9) { return vec4(0.0); }

  vec2 cellSize = vec2(18.0, 25.0);
  vec2 p = vec2(pos.x - u_time * 3.5, pos.y);
  vec2 cellF = floor(p / cellSize);
  ivec2 cell = ivec2(cellF);
  if (hash(cell) < 0.60) { return vec4(0.0); }

  vec2 cellPos = p / cellSize - cellF;
  vec2 starPos = vec2(0.2 + hash(cell + ivec2(17, 3)) * 0.6, 0.2 + hash(cell + ivec2(31, 7)) * 0.6);
  vec2 d = cellPos - starPos;
  d.x *= cellSize.x / cellSize.y;
  float radius = 0.03 + hash(cell + ivec2(91, 11)) * 0.04;
  float intensity = ss(radius, 0.0, length(d));
  if (intensity <= 0.0) { return vec4(0.0); }

  float phase = hash(cell + ivec2(5, 50)) * 6.2831853;
  float freq = 0.35 + hash(cell + ivec2(27, 2)) * 0.55;
  float twinkle = 0.35 + 0.65 * (0.5 + 0.5 * sin(u_time * freq + phase));
  vec3 tint = mix(vec3(0.80, 0.87, 1.0), vec3(1.0, 0.93, 0.82), hash(cell + ivec2(63, 19)));
  float a = intensity * twinkle * u_stars * (1.0 - ss(0.55, 0.90, y01));
  return vec4(tint * a, a);
}

float rainLayer(vec2 p, float phase, float colW, float density, float streakLen, float period, float halfW, int seed) {
  float colF = floor(p.x / colW);
  ivec2 col = ivec2(int(colF), seed);
  if (hash(col + ivec2(0, 41)) > density) { return 0.0; }

  float colOffset = 0.2 + hash(col + ivec2(0, 17)) * 0.6;
  float xDist = abs(p.x - (colF + colOffset) * colW);
  if (xDist > halfW) { return 0.0; }

  float len = streakLen * (0.7 + 0.6 * hash(col + ivec2(0, 29)));
  float y = phase - p.y + hash(col + ivec2(0, 7)) * 1000.0;
  float yMod = fract(y / period) * period;
  if (yMod > len) { return 0.0; }

  float yFall = 1.0 - yMod / len;
  float xFall = 1.0 - xDist / halfW;
  return yFall * xFall * (0.6 + 0.4 * hash(col + ivec2(0, 3)));
}

vec4 rain(vec2 pos) {
  float phase = u_rain.x;
  float intens = clamp(u_rain.y, 0.0, 3.0);
  float slant = u_rain.z;
  float w = clamp(intens / 1.8, 0.0, 1.0);
  float extra = clamp((intens - 1.8) / 1.2, 0.0, 1.0);

  float streakLen = mix(9.0, 52.0, w);
  float period = mix(110.0, 180.0, w);
  float c = cos(slant);
  float s = sin(slant);
  vec2 p = vec2(pos.x * c - pos.y * s, pos.x * s + pos.y * c);

  float dFar = clamp(mix(0.20, 0.80, w) + extra * 0.12, 0.0, 1.0);
  float dMid = clamp(mix(0.16, 0.70, w) + extra * 0.10, 0.0, 1.0);
  float dNear = clamp(mix(0.06, 0.38, w) + extra * 0.10, 0.0, 1.0);

  float far = rainLayer(p, phase * 0.55, 9.0, dFar, streakLen * 0.55, period * 0.55, mix(0.30, 0.45, w), 1);
  float mid = rainLayer(p, phase * 0.85, 16.0, dMid, streakLen * 0.85, period * 0.85, mix(0.45, 0.80, w), 2);
  float near = rainLayer(p, phase * 1.25, 30.0, dNear, streakLen * 1.25, period * 1.25, mix(0.80, 1.50, w), 3);

  vec3 farCol = vec3(0.72, 0.80, 0.94);
  vec3 nearCol = vec3(0.88, 0.92, 0.99);
  float brightness = mix(0.55, 0.80, w);
  float aFar = far * 0.30 * brightness;
  float aMid = mid * 0.50 * brightness;
  float aNear = sqrt(near) * near * 0.55 * brightness;

  vec4 color = vec4(farCol * aFar, aFar);
  color = over(vec4(mix(farCol, nearCol, 0.5) * aMid, aMid), color);
  return over(vec4(nearCol * aNear, aNear), color);
}

float snowLayer(vec2 pos, vec2 fall, float cellSize, float speed, float density, float radius, float softness, float sway, int seed) {
  vec2 p = vec2(pos.x - fall.x * speed, pos.y);
  float colF = floor(p.x / cellSize);
  float colSpeed = 0.8 + 0.4 * hash(ivec2(int(colF), seed * 131 + 7));
  p.y -= fall.y * speed * colSpeed;

  vec2 cellF = floor(p / cellSize);
  ivec2 cell = ivec2(cellF) + ivec2(0, seed * 7919);
  if (hash(cell + ivec2(13, 0)) > density) { return 0.0; }

  vec2 uv = p / cellSize - cellF;
  float r = radius * (0.6 + 0.4 * hash(cell + ivec2(7, 77)));
  float amp = sway * (0.5 + 0.5 * hash(cell + ivec2(9, 99)));
  float margin = r + amp;
  vec2 center = vec2(mix(margin, 1.0 - margin, hash(cell + ivec2(3, 31))), mix(r, 1.0 - r, hash(cell + ivec2(5, 57))));
  float freq = 0.5 + 0.9 * hash(cell + ivec2(11, 13));
  float phase = hash(cell + ivec2(23, 29)) * 6.2831853;
  center.x += sin(u_time * freq + phase) * amp;

  float dist = length(uv - center);
  float a = 1.0 - ss(r * (1.0 - softness), r, dist);
  return a * (0.55 + 0.45 * hash(cell + ivec2(2, 22)));
}

vec4 snow(vec2 pos) {
  vec2 fall = u_snow.xy;
  float intens = clamp(u_snow.z, 0.3, 3.0);
  float gust = 1.0 + min(abs(u_snow.w), 10.0) * 0.05;

  float far = snowLayer(pos, fall, 14.0, 0.55, clamp(intens * 0.50, 0.0, 1.0), 0.11, 0.45, 0.10 * gust, 1);
  float mid = snowLayer(pos, fall, 30.0, 1.00, clamp(intens * 0.45, 0.0, 1.0), 0.10, 0.60, 0.12 * gust, 2);
  float near = snowLayer(pos, fall, 90.0, 1.70, clamp(intens * 0.16, 0.0, 1.0), 0.085, 1.0, 0.08 * gust, 3);

  float a = min(far * 0.50 + mid * 0.80 + near * 0.50, 1.0) * 0.9;
  return vec4(vec3(a), a);
}

void main() {
  vec2 pos = vec2(gl_FragCoord.x, u_canvas.y - gl_FragCoord.y) / u_scale;
  vec4 color = stars(pos);
  color = over(texture(u_sky, gl_FragCoord.xy / u_canvas), color);
  if (u_precipitation == 1) {
    color = over(rain(pos), color);
  } else if (u_precipitation == 2) {
    color = over(snow(pos), color);
  }
  if (u_flash > 0.0) {
    color = over(vec4(u_flash), color);
  }
  color.a = clamp(color.a, 0.0, 1.0);
  fragColor = vec4(min(color.rgb, vec3(color.a)), color.a);
}
