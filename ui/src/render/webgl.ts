export interface Camera {
  cellSize: number;
  originX: number;
  originY: number;
}

export interface RgbColors {
  alive: [number, number, number];
  dead: [number, number, number];
  bg: [number, number, number];
  grid: [number, number, number];
}

export interface FramePayload {
  width: number;
  height: number;
  rowBytes: number;
  payload: Uint8Array;
}

export interface Renderer {
  upload(frame: FramePayload): void;
  draw(camera: Camera, gridLines: boolean): void;
  gridWidth(): number;
  gridHeight(): number;
  hasFrame(): boolean;
  dispose(): void;
}

const VERT = `#version 300 es
in vec2 a_pos;
void main() {
  gl_Position = vec4(a_pos, 0.0, 1.0);
}`;

const FRAG = `#version 300 es
precision highp float;
precision highp int;
precision highp usampler2D;

uniform usampler2D u_grid;
uniform vec2 u_canvas;     // device px
uniform float u_cell;      // device px per cell
uniform vec2 u_origin;     // device px of cell (0,0)
uniform int u_width;
uniform int u_height;
uniform vec3 u_alive;
uniform vec3 u_dead;
uniform vec3 u_bg;
uniform vec3 u_gridcol;
uniform int u_gridlines;

out vec4 fragColor;

void main() {
  // gl_FragCoord origin is bottom-left; flip Y to top-left screen space.
  vec2 px = vec2(gl_FragCoord.x, u_canvas.y - gl_FragCoord.y);
  vec2 cellf = (px - u_origin) / u_cell;

  if (cellf.x < 0.0 || cellf.y < 0.0) {
    fragColor = vec4(u_bg, 1.0);
    return;
  }
  int cx = int(floor(cellf.x));
  int cy = int(floor(cellf.y));
  if (cx >= u_width || cy >= u_height) {
    fragColor = vec4(u_bg, 1.0);
    return;
  }

  uint byteVal = texelFetch(u_grid, ivec2(cx >> 3, cy), 0).r;
  uint alive = (byteVal >> uint(cx & 7)) & 1u;
  vec3 col = alive == 1u ? u_alive : u_dead;

  if (u_gridlines == 1 && u_cell >= 6.0) {
    vec2 f = fract(cellf);
    float lw = 1.0 / u_cell;
    if (f.x < lw || f.y < lw) col = mix(col, u_gridcol, 0.5);
  }
  fragColor = vec4(col, 1.0);
}`;

function compile(
  gl: WebGL2RenderingContext,
  type: number,
  src: string,
): WebGLShader {
  const sh = gl.createShader(type)!;
  gl.shaderSource(sh, src);
  gl.compileShader(sh);
  if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)) {
    const log = gl.getShaderInfoLog(sh);
    gl.deleteShader(sh);
    throw new Error(`shader compile failed: ${log}`);
  }
  return sh;
}

function link(
  gl: WebGL2RenderingContext,
  vs: WebGLShader,
  fs: WebGLShader,
): WebGLProgram {
  const prog = gl.createProgram()!;
  gl.attachShader(prog, vs);
  gl.attachShader(prog, fs);
  gl.linkProgram(prog);
  if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) {
    const log = gl.getProgramInfoLog(prog);
    gl.deleteProgram(prog);
    throw new Error(`program link failed: ${log}`);
  }
  return prog;
}

export function createRenderer(
  canvas: HTMLCanvasElement,
  colors: RgbColors,
): Renderer | null {
  const gl = canvas.getContext("webgl2", {
    antialias: false,
    premultipliedAlpha: false,
  });
  if (!gl) return null;

  const vs = compile(gl, gl.VERTEX_SHADER, VERT);
  const fs = compile(gl, gl.FRAGMENT_SHADER, FRAG);
  const prog = link(gl, vs, fs);
  gl.deleteShader(vs);
  gl.deleteShader(fs);

  const vao = gl.createVertexArray()!;
  gl.bindVertexArray(vao);
  const buf = gl.createBuffer()!;
  gl.bindBuffer(gl.ARRAY_BUFFER, buf);
  gl.bufferData(
    gl.ARRAY_BUFFER,
    new Float32Array([-1, -1, 3, -1, -1, 3]),
    gl.STATIC_DRAW,
  );
  const aPos = gl.getAttribLocation(prog, "a_pos");
  gl.enableVertexAttribArray(aPos);
  gl.vertexAttribPointer(aPos, 2, gl.FLOAT, false, 0, 0);
  gl.bindVertexArray(null);

  const tex = gl.createTexture()!;
  gl.bindTexture(gl.TEXTURE_2D, tex);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);

  const u = {
    grid: gl.getUniformLocation(prog, "u_grid"),
    canvas: gl.getUniformLocation(prog, "u_canvas"),
    cell: gl.getUniformLocation(prog, "u_cell"),
    origin: gl.getUniformLocation(prog, "u_origin"),
    width: gl.getUniformLocation(prog, "u_width"),
    height: gl.getUniformLocation(prog, "u_height"),
    alive: gl.getUniformLocation(prog, "u_alive"),
    dead: gl.getUniformLocation(prog, "u_dead"),
    bg: gl.getUniformLocation(prog, "u_bg"),
    gridcol: gl.getUniformLocation(prog, "u_gridcol"),
    gridlines: gl.getUniformLocation(prog, "u_gridlines"),
  };

  let gw = 0;
  let gh = 0;

  return {
    upload(frame: FramePayload) {
      gl.bindTexture(gl.TEXTURE_2D, tex);
      gl.pixelStorei(gl.UNPACK_ALIGNMENT, 1);
      gl.texImage2D(
        gl.TEXTURE_2D,
        0,
        gl.R8UI,
        frame.rowBytes,
        frame.height,
        0,
        gl.RED_INTEGER,
        gl.UNSIGNED_BYTE,
        frame.payload,
      );
      gw = frame.width;
      gh = frame.height;
    },

    draw(camera: Camera, gridLines: boolean) {
      if (gw === 0) return;
      gl.viewport(0, 0, canvas.width, canvas.height);
      gl.clearColor(colors.bg[0], colors.bg[1], colors.bg[2], 1);
      gl.clear(gl.COLOR_BUFFER_BIT);

      gl.useProgram(prog);
      gl.bindVertexArray(vao);
      gl.activeTexture(gl.TEXTURE0);
      gl.bindTexture(gl.TEXTURE_2D, tex);

      gl.uniform1i(u.grid, 0);
      gl.uniform2f(u.canvas, canvas.width, canvas.height);
      gl.uniform1f(u.cell, camera.cellSize);
      gl.uniform2f(u.origin, camera.originX, camera.originY);
      gl.uniform1i(u.width, gw);
      gl.uniform1i(u.height, gh);
      gl.uniform3fv(u.alive, colors.alive);
      gl.uniform3fv(u.dead, colors.dead);
      gl.uniform3fv(u.bg, colors.bg);
      gl.uniform3fv(u.gridcol, colors.grid);
      gl.uniform1i(u.gridlines, gridLines ? 1 : 0);

      gl.drawArrays(gl.TRIANGLES, 0, 3);
      gl.bindVertexArray(null);
    },

    gridWidth: () => gw,
    gridHeight: () => gh,
    hasFrame: () => gw > 0,

    dispose() {
      gl.deleteTexture(tex);
      gl.deleteBuffer(buf);
      gl.deleteVertexArray(vao);
      gl.deleteProgram(prog);
    },
  };
}
