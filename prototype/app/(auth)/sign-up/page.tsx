import { redirect } from "next/navigation"

// Sign-up and sign-in are now a single email-first door: a new email is
// recognized and routed through verification automatically from /sign-in, and
// workspace details are collected in onboarding. This route is kept only so
// existing links and bookmarks don't 404.
export default function SignUpPage() {
  redirect("/sign-in")
}
