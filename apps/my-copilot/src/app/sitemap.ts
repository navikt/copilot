import type { MetadataRoute } from "next";
import { getNewsItems } from "@/lib/news";

const BASE_URL = "https://ki-utvikling.nav.no";

export default function sitemap(): MetadataRoute.Sitemap {
  const news = getNewsItems();

  const staticPages: MetadataRoute.Sitemap = [
    { url: BASE_URL, changeFrequency: "weekly", priority: 1.0 },
    { url: `${BASE_URL}/nyheter`, changeFrequency: "weekly", priority: 0.9 },
    { url: `${BASE_URL}/praksis`, changeFrequency: "monthly", priority: 0.8 },
    { url: `${BASE_URL}/verktoy`, changeFrequency: "monthly", priority: 0.8 },
    { url: `${BASE_URL}/ordliste`, changeFrequency: "monthly", priority: 0.7 },
    { url: `${BASE_URL}/nav-pilot`, changeFrequency: "monthly", priority: 0.8 },
    { url: `${BASE_URL}/install`, changeFrequency: "monthly", priority: 0.7 },
    { url: `${BASE_URL}/personvern`, changeFrequency: "yearly", priority: 0.3 },
    { url: `${BASE_URL}/tilgjengelighet`, changeFrequency: "yearly", priority: 0.3 },
  ];

  const newsPages: MetadataRoute.Sitemap = news.map((item) => ({
    url: `${BASE_URL}/nyheter/${item.slug}`,
    changeFrequency: "monthly" as const,
    priority: 0.6,
  }));

  return [...staticPages, ...newsPages];
}
