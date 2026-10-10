import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  BarElement,
  ArcElement,
  Filler,
} from "chart.js";

// Register Chart.js components once
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  BarElement,
  ArcElement,
  Filler
);

// GitHub Insights-style color palette
export const chartColors = [
  "rgba(59, 130, 246, 1)", // blue
  "rgba(16, 185, 129, 1)", // green
  "rgba(139, 92, 246, 1)", // purple
  "rgba(245, 158, 11, 1)", // amber
  "rgba(239, 68, 68, 1)", // red
  "rgba(107, 114, 128, 1)", // gray
  "rgba(236, 72, 153, 1)", // pink
  "rgba(6, 182, 212, 1)", // cyan
];

/**
 * Reads an Aksel colour token (without the `--ax-` prefix) from the page, so a chart follows the design system.
 * Chart.js draws on a canvas and cannot use CSS variables. Pass it as a scriptable option, e.g.
 * `color: () => axColor("text-neutral-subtle")`, so it is read at draw time in the browser.
 */
export function axColor(token: string, alpha = 1): string {
  if (typeof document === "undefined") return "";
  let value = getComputedStyle(document.body).getPropertyValue(`--ax-${token}`).trim();
  if (alpha >= 1) return value;
  if (/^#[0-9a-f]{3}$/i.test(value)) value = "#" + [...value.slice(1)].map((c) => c + c).join("");
  if (!/^#[0-9a-f]{6}$/i.test(value)) return value;
  return (
    value +
    Math.round(alpha * 255)
      .toString(16)
      .padStart(2, "0")
  );
}

// Aksel tokens for categorical series, in the same hue order as chartColors.
export const axSeries = [
  "accent-600",
  "success-600",
  "meta-purple-600",
  "warning-600",
  "danger-600",
  "neutral-600",
  "meta-lime-600",
  "info-600",
];

/** Series colour i as a scriptable chart.js option. */
export const seriesColor =
  (i: number, alpha = 1) =>
  () =>
    axColor(axSeries[i % axSeries.length], alpha);

// Helper to get background color with opacity
export const getBackgroundColor = (color: string, opacity: number = 0.1): string => {
  return color.replace("1)", `${opacity})`);
};

// GitHub-style grid options
const githubGridStyle = {
  color: () => axColor("border-neutral-subtle", 0.4),
  drawBorder: false,
};

const githubTickStyle = {
  color: () => axColor("text-neutral-subtle"),
  font: { size: 11 },
};

// Common chart options with GitHub styling
export const commonLineOptions = {
  responsive: true,
  // The wrapper sets the height (see chartBoxClass); a fixed aspect ratio made plots ~60 px tall on phones.
  maintainAspectRatio: false,
  interaction: {
    mode: "index" as const,
    intersect: false,
  },
  plugins: {
    legend: {
      position: "top" as const,
      labels: {
        usePointStyle: true,
        pointStyle: "circle",
        padding: 20,
        font: { size: 12 },
      },
    },
    tooltip: {
      backgroundColor: () => axColor("bg-neutral-strong"),
      padding: 12,
      titleFont: { size: 13 },
      bodyFont: { size: 12 },
      cornerRadius: 8,
    },
  },
  scales: {
    y: {
      beginAtZero: true,
      border: { display: false },
      grid: githubGridStyle,
      ticks: githubTickStyle,
    },
    x: {
      border: { display: false },
      grid: { display: false },
      ticks: githubTickStyle,
    },
  },
};

// Common chart wrapper styling
export const chartWrapperClass = "bg-(--ax-bg-default) p-4 rounded-lg border border-(--ax-border-neutral-subtle)";

// Height for a chart canvas: room for the plot on a phone, a little more on wide screens.
export const chartBoxClass = "relative h-64 md:h-80";

// Compact legend below the plot; chart.js wraps the items onto more rows when needed.
export const bottomLegend = {
  position: "bottom" as const,
  labels: { usePointStyle: true, pointStyle: "circle", boxWidth: 8, boxHeight: 8, padding: 10, font: { size: 11 } },
};

// Default no data message
export const NO_DATA_MESSAGE = "Ingen data tilgjengelig for visning";

// Horizontal bar chart options
export const commonHorizontalBarOptions = {
  responsive: true,
  maintainAspectRatio: false,
  indexAxis: "y" as const,
  plugins: {
    legend: {
      display: false,
    },
    tooltip: {
      backgroundColor: () => axColor("bg-neutral-strong"),
      padding: 12,
      titleFont: { size: 13 },
      bodyFont: { size: 12 },
      cornerRadius: 8,
    },
  },
  scales: {
    y: {
      border: { display: false },
      grid: { display: false },
      ticks: githubTickStyle,
    },
    x: {
      beginAtZero: true,
      border: { display: false },
      grid: githubGridStyle,
      ticks: githubTickStyle,
    },
  },
};
