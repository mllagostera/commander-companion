/**
 * The backend doesn't expose a per-user color — a fixed palette is assigned by
 * position within the group, consistent as long as the member list doesn't
 * change order (same idea as the mock's avatars, colored by hand).
 */
const PALETTE = ['#818cf8', '#c4b5fd', '#7c3aed', '#d8b4fe', '#a78bfa', '#9333ea', '#6d28d9']

export function avatarColor(index: number): string {
  return PALETTE[index % PALETTE.length]!
}
