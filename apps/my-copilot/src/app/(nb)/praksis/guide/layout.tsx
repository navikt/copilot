"use client";

import NextLink from "next/link";
import { usePathname } from "next/navigation";
import { categories } from "../data";

export default function GuideLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  return (
    <div className="max-w-7xl mx-auto flex gap-12 px-4 sm:px-8 md:px-12 py-12">
      <aside className="hidden lg:block w-64 shrink-0">
        <div className="sticky top-24">
          <NextLink href="/praksis" className="text-blue-600 hover:underline mb-8 block font-medium">
            ← Tilbake til oversikt
          </NextLink>
          <nav className="space-y-8">
            {categories.map((cat) => (
              <div key={cat.title}>
                <h3 className="font-semibold text-sm text-gray-900 uppercase tracking-wider mb-3">{cat.title}</h3>
                <ul className="space-y-3">
                  {cat.guides.map((guide) => {
                    const href = `/praksis/guide/${guide.id}`;
                    const isActive = pathname === href;
                    return (
                      <li key={guide.id}>
                        <NextLink
                          href={href}
                          className={`text-sm block ${isActive ? "text-blue-600 font-semibold" : "text-gray-600 hover:text-blue-600"}`}
                        >
                          {guide.title}
                        </NextLink>
                      </li>
                    );
                  })}
                </ul>
              </div>
            ))}
          </nav>
        </div>
      </aside>
      <main className="min-w-0 flex-1">{children}</main>
    </div>
  );
}
