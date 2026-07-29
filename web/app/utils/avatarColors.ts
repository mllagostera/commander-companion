/**
 * El backend no expone un color por usuario — se asigna una paleta fija por
 * posición dentro del grupo, consistente mientras la lista de miembros no
 * cambie de orden (misma idea que los avatares del mock, coloreados a mano).
 */
const PALETTE = ['#818cf8', '#c4b5fd', '#7c3aed', '#d8b4fe', '#a78bfa', '#9333ea', '#6d28d9']

export function avatarColor(index: number): string {
  return PALETTE[index % PALETTE.length]!
}
