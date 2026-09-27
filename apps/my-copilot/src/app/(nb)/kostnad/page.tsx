// Redirect /kostnad to /statistikk (cost tab is now integrated there)
import { permanentRedirect } from "next/navigation";
import { getUser } from "@/lib/auth";

export default async function KostnadRedirect() {
  await getUser();
  permanentRedirect("/statistikk#m%C3%A5ned-hittil-modeller-og-kostnad");
}
