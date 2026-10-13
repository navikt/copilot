import type { ReactNode } from "react";
import { requireAdmin } from "@/lib/auth";

// Every page under /admin answers 404 to everyone but admins (COPILOT_ADMIN_GROUP_ID).
export default async function AdminLayout({ children }: { children: ReactNode }) {
  await requireAdmin();
  return children;
}
