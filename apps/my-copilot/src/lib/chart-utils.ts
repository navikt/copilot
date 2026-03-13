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

// Helper to get background color with opacity
export const getBackgroundColor = (color: string, opacity: number = 0.1): string => {
  return color.replace("1)", `${opacity})`);
};

// Create gradient for area charts
export const createGradient = (ctx: CanvasRenderingContext2D, color: string, height: number = 300): CanvasGradient => {
  const gradient = ctx.createLinearGradient(0, 0, 0, height);
  gradient.addColorStop(0, color.replace("1)", "0.3)"));
  gradient.addColorStop(1, color.replace("1)", "0.02)"));
  return gradient;
};

// GitHub-style grid options
const githubGridStyle = {
  color: "rgba(0, 0, 0, 0.06)",
  drawBorder: false,
};

const githubTickStyle = {
  color: "#6B7280",
  font: { size: 11 },
};

// Common chart options with GitHub styling
export const commonLineOptions = {
  responsive: true,
  maintainAspectRatio: true,
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
      backgroundColor: "rgba(0, 0, 0, 0.8)",
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

// Donut chart options
export const commonDonutOptions = {
  responsive: true,
  maintainAspectRatio: true,
  cutout: "60%",
  plugins: {
    legend: {
      position: "right" as const,
      labels: {
        usePointStyle: true,
        pointStyle: "circle",
        padding: 16,
        font: { size: 12 },
      },
    },
    tooltip: {
      backgroundColor: "rgba(0, 0, 0, 0.8)",
      padding: 12,
      cornerRadius: 8,
    },
  },
};

// Common chart wrapper styling
export const chartWrapperClass = "bg-white p-4 rounded-lg border border-gray-200";

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
      backgroundColor: "rgba(0, 0, 0, 0.8)",
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
      ticks: {
        color: "#6B7280",
        font: { size: 11 },
      },
    },
    x: {
      beginAtZero: true,
      border: { display: false },
      grid: {
        color: "rgba(0, 0, 0, 0.06)",
        drawBorder: false,
      },
      ticks: {
        color: "#6B7280",
        font: { size: 11 },
      },
    },
  },
};
