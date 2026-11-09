/**
 * SVG paths for a sparkline. `slots` is how many points fit the width, so the
 * line grows in from the right until the buffer is full. max 0 = auto-scale.
 */
export function sparkPath(values: number[], width: number, height: number, max: number, slots: number) {
  if (values.length === 0) return { line: '', area: '' };
  const top = max > 0 ? max : Math.max(...values, 0) || 1;
  const step = width / Math.max(slots - 1, 1);
  const x0 = width - step * (values.length - 1);
  const pts = values.map((v, i) => {
    const x = Math.round((x0 + i * step) * 100) / 100;
    const y = Math.round((height - (Math.min(Math.max(v, 0), top) / top) * height) * 100) / 100;
    return `${x} ${y}`;
  });
  const line = 'M' + pts.join('L');
  const first = pts[0]!.split(' ')[0];
  const area = `${line}L${width} ${height}L${first} ${height}Z`;
  return { line, area };
}
