// Klijentska validacija naziva - ista pravila kao ValidateName u Go servisu.
const FORBIDDEN = /[\/\\:*?"<>|\u0000-\u001f\u007f]/;
export const MAX_NAME_LENGTH = 255;

/** Vraca poruku o gresci ili null ako je naziv validan. */
export function validateName(raw: string): string | null {
  const name = raw.trim();
  if (!name) return 'Naziv je obavezan.';
  if (name === '.' || name === '..') return 'Naziv nije dozvoljen.';
  if ([...name].length > MAX_NAME_LENGTH) return `Naziv moze imati najvise ${MAX_NAME_LENGTH} karaktera.`;
  if (FORBIDDEN.test(name)) return 'Naziv ne sme sadrzati znakove  / \\ : * ? " < > |';
  return null;
}
