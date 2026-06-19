import { redirect } from "next/navigation";

/**
 * Root-Route: leitet direkt zum Dashboard weiter.
 * Auth-Check erfolgt im Middleware (kommt mit US-001).
 */
export default function RootPage() {
  redirect("/dashboard");
}
