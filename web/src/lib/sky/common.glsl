uint hashBits(ivec2 p) {
  uvec2 x = uvec2(p);
  uvec2 q = 1103515245u * ((x >> 1u) ^ x.yx);
  return 1103515245u * (q.x ^ (q.y >> 3u));
}

float hash(ivec2 p) {
  return float(hashBits(p)) * (1.0 / 4294967295.0);
}

float ss(float edge0, float edge1, float x) {
  float t = clamp((x - edge0) / (edge1 - edge0), 0.0, 1.0);
  return t * t * (3.0 - 2.0 * t);
}

vec4 over(vec4 top, vec4 bottom) {
  return top + bottom * (1.0 - top.a);
}
