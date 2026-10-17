import commonSource from "./common.glsl?raw";
import compositeSource from "./composite.frag?raw";
import skySource from "./sky.frag?raw";
import type { Rgb, Vec2 } from "./state";

const HEADER =
  "#version 300 es\nprecision highp float;\nprecision highp int;\n";

const VERTEX_SOURCE = `#version 300 es
void main() {
  vec2 p = vec2(float((gl_VertexID << 1) & 2), float(gl_VertexID & 2));
  gl_Position = vec4(p * 2.0 - 1.0, 0.0, 1.0);
}`;

const SKY_TEXEL_BUDGET = 500_000;
const CANVAS_PIXEL_BUDGET = 4_000_000;

const CONTEXT_ATTRIBUTES: WebGLContextAttributes = {
  alpha: true,
  premultipliedAlpha: true,
  antialias: false,
  depth: false,
  stencil: false,
  powerPreference: "low-power",
  failIfMajorPerformanceCaveat: true,
};

export type Precipitation = "none" | "rain" | "snow";

export type SkyFrame = {
  time: number;
  drift: Vec2;
  clouds: [cumulus: number, deck: number, cirrus: number, absorption: number];
  light: [sun: number, sunFade: number, night: number, occlusion: number];
  lit: Rgb;
  shadow: Rgb;
  flash: number;
  flashX: number;
  stars: number;
  precipitation: Precipitation;
  rain: [phase: number, intensity: number, slant: number];
  snow: [fallX: number, fallY: number, intensity: number, wind: number];
};

type Program = {
  program: WebGLProgram;
  uniforms: Record<string, WebGLUniformLocation | null>;
};

function compile(
  gl: WebGL2RenderingContext,
  type: number,
  source: string,
): WebGLShader {
  const shader = gl.createShader(type);
  if (!shader) throw new Error("sky: cannot create shader");
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (
    !gl.getShaderParameter(shader, gl.COMPILE_STATUS) &&
    !gl.isContextLost()
  ) {
    const log = gl.getShaderInfoLog(shader);
    gl.deleteShader(shader);
    throw new Error(`sky: shader compile failed: ${log}`);
  }
  return shader;
}

function link(
  gl: WebGL2RenderingContext,
  fragmentBody: string,
  names: string[],
): Program {
  const program = gl.createProgram();
  if (!program) throw new Error("sky: cannot create program");
  const vertex = compile(gl, gl.VERTEX_SHADER, VERTEX_SOURCE);
  const fragment = compile(
    gl,
    gl.FRAGMENT_SHADER,
    HEADER + commonSource + fragmentBody,
  );
  gl.attachShader(program, vertex);
  gl.attachShader(program, fragment);
  gl.linkProgram(program);
  gl.deleteShader(vertex);
  gl.deleteShader(fragment);
  if (!gl.getProgramParameter(program, gl.LINK_STATUS) && !gl.isContextLost()) {
    throw new Error(
      `sky: program link failed: ${gl.getProgramInfoLog(program)}`,
    );
  }
  const uniforms = Object.fromEntries(
    names.map((name) => [name, gl.getUniformLocation(program, name)]),
  );
  return { program, uniforms };
}

export class SkyRenderer {
  private readonly gl: WebGL2RenderingContext;
  private sky!: Program;
  private composite!: Program;
  private vao: WebGLVertexArrayObject | null = null;
  private texture: WebGLTexture | null = null;
  private framebuffer: WebGLFramebuffer | null = null;
  private cssWidth = 0;
  private cssHeight = 0;
  private skyWidth = 0;
  private skyHeight = 0;

  static create(canvas: HTMLCanvasElement): SkyRenderer | null {
    const gl = canvas.getContext("webgl2", CONTEXT_ATTRIBUTES);
    return gl ? new SkyRenderer(canvas, gl) : null;
  }

  private constructor(
    private readonly canvas: HTMLCanvasElement,
    gl: WebGL2RenderingContext,
  ) {
    this.gl = gl;
    this.setup();
  }

  get width(): number {
    return this.cssWidth;
  }

  get height(): number {
    return this.cssHeight;
  }

  setup(): void {
    const gl = this.gl;
    this.sky = link(gl, skySource, [
      "u_size",
      "u_time",
      "u_drift",
      "u_clouds",
      "u_light",
      "u_lit",
      "u_shadow",
      "u_flash",
    ]);
    this.composite = link(gl, compositeSource, [
      "u_sky",
      "u_canvas",
      "u_scale",
      "u_size",
      "u_time",
      "u_stars",
      "u_precipitation",
      "u_rain",
      "u_snow",
      "u_flash",
    ]);
    this.vao = gl.createVertexArray();
    this.texture = gl.createTexture();
    gl.bindTexture(gl.TEXTURE_2D, this.texture);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    this.framebuffer = gl.createFramebuffer();
    this.skyWidth = 0;
    this.skyHeight = 0;
    if (this.cssWidth > 0 && this.cssHeight > 0)
      this.resize(this.cssWidth, this.cssHeight);
  }

  resize(cssWidth: number, cssHeight: number): void {
    this.cssWidth = cssWidth;
    this.cssHeight = cssHeight;
    const area = cssWidth * cssHeight;
    if (area <= 0) return;

    let ratio = Math.min(window.devicePixelRatio || 1, 2);
    if (area * ratio * ratio > CANVAS_PIXEL_BUDGET)
      ratio = Math.sqrt(CANVAS_PIXEL_BUDGET / area);
    this.canvas.width = Math.max(1, Math.round(cssWidth * ratio));
    this.canvas.height = Math.max(1, Math.round(cssHeight * ratio));

    const skyScale = Math.min(1, Math.sqrt(SKY_TEXEL_BUDGET / area));
    const skyWidth = Math.max(1, Math.round(cssWidth * skyScale));
    const skyHeight = Math.max(1, Math.round(cssHeight * skyScale));
    if (skyWidth === this.skyWidth && skyHeight === this.skyHeight) return;
    this.skyWidth = skyWidth;
    this.skyHeight = skyHeight;

    const gl = this.gl;
    gl.bindTexture(gl.TEXTURE_2D, this.texture);
    gl.texImage2D(
      gl.TEXTURE_2D,
      0,
      gl.RGBA8,
      skyWidth,
      skyHeight,
      0,
      gl.RGBA,
      gl.UNSIGNED_BYTE,
      null,
    );
    gl.bindFramebuffer(gl.FRAMEBUFFER, this.framebuffer);
    gl.framebufferTexture2D(
      gl.FRAMEBUFFER,
      gl.COLOR_ATTACHMENT0,
      gl.TEXTURE_2D,
      this.texture,
      0,
    );
    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
  }

  render(frame: SkyFrame, renderSky: boolean): void {
    const gl = this.gl;
    if (gl.isContextLost() || this.skyWidth === 0) return;
    gl.bindVertexArray(this.vao);

    if (renderSky) {
      const { program, uniforms: u } = this.sky;
      gl.bindFramebuffer(gl.FRAMEBUFFER, this.framebuffer);
      gl.viewport(0, 0, this.skyWidth, this.skyHeight);
      gl.useProgram(program);
      gl.uniform2f(u.u_size, this.skyWidth, this.skyHeight);
      gl.uniform1f(u.u_time, frame.time);
      gl.uniform2f(u.u_drift, frame.drift[0], frame.drift[1]);
      gl.uniform4f(u.u_clouds, ...frame.clouds);
      gl.uniform4f(u.u_light, ...frame.light);
      gl.uniform3f(u.u_lit, ...frame.lit);
      gl.uniform3f(u.u_shadow, ...frame.shadow);
      gl.uniform2f(u.u_flash, frame.flash, frame.flashX);
      gl.drawArrays(gl.TRIANGLES, 0, 3);
    }

    const { program, uniforms: u } = this.composite;
    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
    gl.viewport(0, 0, this.canvas.width, this.canvas.height);
    gl.useProgram(program);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, this.texture);
    gl.uniform1i(u.u_sky, 0);
    gl.uniform2f(u.u_canvas, this.canvas.width, this.canvas.height);
    gl.uniform1f(u.u_scale, this.canvas.width / this.cssWidth);
    gl.uniform2f(u.u_size, this.cssWidth, this.cssHeight);
    gl.uniform1f(u.u_time, frame.time);
    gl.uniform1f(u.u_stars, frame.stars);
    gl.uniform1i(
      u.u_precipitation,
      frame.precipitation === "rain"
        ? 1
        : frame.precipitation === "snow"
          ? 2
          : 0,
    );
    gl.uniform3f(u.u_rain, ...frame.rain);
    gl.uniform4f(u.u_snow, ...frame.snow);
    gl.uniform1f(u.u_flash, frame.flash * 0.12);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
  }
}
