// Redirect /kostnad to /statistikk (cost tab is now integrated there)
import { redirect } from "next/navigation";
import { getUser } from "@/lib/auth";

export default async function KostnadRedirect() {
  await getUser();
  redirect("/statistikk#kostnad");
}
