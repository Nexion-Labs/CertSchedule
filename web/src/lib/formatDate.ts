function pad(n: number, width = 2): string {
  return n.toString().padStart(width, "0");
}

export function formatDateTime(value: string | number | Date): string {
  const d = new Date(value);
  const day = pad(d.getDate());
  const month = pad(d.getMonth() + 1);
  const year = d.getFullYear();
  const hours = pad(d.getHours());
  const minutes = pad(d.getMinutes());
  const seconds = pad(d.getSeconds());
  return `${day}/${month}/${year} ${hours}:${minutes}:${seconds}`;
}
