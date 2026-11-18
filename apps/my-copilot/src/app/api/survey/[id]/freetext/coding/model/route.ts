import { forwardCoding } from "../forward";

/** Imports the codes file `copilot-survey code` writes: hashes of masked texts and their codes, no text. */
export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  return forwardCoding(request, (await params).id, "coding/model", 4 << 20);
}
