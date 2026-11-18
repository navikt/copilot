import { forwardCoding } from "./forward";

/** Saves the person's codes for one text, or a decision where the coders disagree. */
export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  return forwardCoding(request, (await params).id, "coding", 4 << 10);
}
