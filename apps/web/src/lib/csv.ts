/** A cell starting with = + - @ would run as a formula when opened in Excel. */
export function csvCell(v: string): string {
  const safe = v !== "" && "=+-@\t\r".includes(v[0]!) ? `'${v}` : v;
  return `"${safe.replaceAll('"', '""')}"`;
}

export function downloadCsv(filename: string, rows: string[][]) {
  const text = "﻿" + rows.map((r) => r.map(csvCell).join(",")).join("\r\n") + "\r\n";
  const url = URL.createObjectURL(new Blob([text], { type: "text/csv;charset=utf-8" }));
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
