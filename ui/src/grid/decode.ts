// Decoder for the engine's bit-packed grid frame (see engine/internal/gridcodec).
// Layout: 24-byte little-endian header then packed bits.

export interface GridFrame {
  width: number;
  height: number;
  wordsPerRow: number;
  /** bytes per row = wordsPerRow * 8 (the texture width). */
  rowBytes: number;
  codec: number;
  /** rowBytes * height bytes of packed bits (1 bit/cell, LSB = column 0). */
  payload: Uint8Array;
}

const HEADER_SIZE = 24;
// "B3S2"
const MAGIC = [0x42, 0x33, 0x53, 0x32];
const CODEC_RAW = 1;

export function decodeFrame(buf: ArrayBuffer): GridFrame {
  if (buf.byteLength < HEADER_SIZE) {
    throw new Error("grid frame shorter than header");
  }
  const dv = new DataView(buf);
  for (let i = 0; i < 4; i++) {
    if (dv.getUint8(i) !== MAGIC[i]) throw new Error("grid frame: bad magic");
  }
  const version = dv.getUint8(4);
  if (version !== 1)
    throw new Error(`grid frame: unsupported version ${version}`);

  const codec = dv.getUint8(5);
  if (codec !== CODEC_RAW) {
    throw new Error(`grid frame: unsupported codec ${codec} (expected raw)`);
  }

  const width = dv.getUint32(8, true);
  const height = dv.getUint32(12, true);
  const wordsPerRow = dv.getUint32(16, true);
  const rowBytes = wordsPerRow * 8;
  const expected = rowBytes * height;

  if (buf.byteLength < HEADER_SIZE + expected) {
    throw new Error(
      `grid frame: payload too short (have ${buf.byteLength - HEADER_SIZE}, want ${expected})`,
    );
  }
  const payload = new Uint8Array(buf, HEADER_SIZE, expected);
  return { width, height, wordsPerRow, rowBytes, codec, payload };
}
