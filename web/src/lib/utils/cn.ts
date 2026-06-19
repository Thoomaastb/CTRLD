import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

/**
 * Kombiniert clsx und tailwind-merge.
 * Verhindert Klassen-Konflikte bei konditionalen Tailwind-Klassen.
 *
 * @example cn("px-4 py-2", condition && "bg-red-500", "px-6") → "py-2 bg-red-500 px-6"
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
