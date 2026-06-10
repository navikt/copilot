import type { ReactNode } from "react";

type Props = {
  children: ReactNode;
};

export default function VideoDetailLayout({ children }: Props) {
  return <div className="flex min-h-full flex-col bg-black">{children}</div>;
}
