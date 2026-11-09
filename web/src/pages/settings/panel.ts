export interface PanelInfo {
  port: number;
  path: string;
  subPort: number;
  pinned: boolean;
  address: string;
}

/** Where the panel answers after a move: same scheme and host, new port and path. */
export function panelURL(loc: Pick<Location, 'protocol' | 'hostname'>, port: number, path: string): string {
  const host = loc.hostname.includes(':') ? `[${loc.hostname}]` : loc.hostname;
  return `${loc.protocol}//${host}:${port}/${path}/`;
}

/** A 16-character [a-z0-9] path, like the one generated at install. */
export function randomPath(n = 16, random: (n: number) => number = cryptoInt): string {
  const alphabet = 'abcdefghijklmnopqrstuvwxyz0123456789';
  let s = '';
  for (let i = 0; i < n; i++) s += alphabet[random(alphabet.length)];
  return s;
}

function cryptoInt(n: number): number {
  const buf = new Uint32Array(1);
  const limit = Math.floor(0x1_0000_0000 / n) * n;
  do crypto.getRandomValues(buf);
  while (buf[0]! >= limit);
  return buf[0]! % n;
}
