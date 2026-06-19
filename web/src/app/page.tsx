import { redirect } from "next/navigation";

/**
 * Root-Route: leitet zum Login weiter.
 * Auth-Middleware prüft ob Setup nötig ist (kommt in nächster Iteration).
 */
export default function RootPage() {
  redirect("/login");
}
