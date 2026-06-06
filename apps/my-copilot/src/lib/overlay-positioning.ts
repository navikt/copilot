import type React from "react";

export function getAnchorStyles(anchor: string): React.CSSProperties {
  const baseStyles: React.CSSProperties = {
    position: "absolute",
  };

  switch (anchor) {
    case "top-left":
      return { ...baseStyles, top: "0.5rem", left: "0.5rem" };
    case "top-right":
      return { ...baseStyles, top: "0.5rem", right: "0.5rem" };
    case "center-left":
      return {
        ...baseStyles,
        top: "50%",
        left: "0.5rem",
        transform: "translateY(-50%)",
      };
    case "center-right":
      return {
        ...baseStyles,
        top: "50%",
        right: "0.5rem",
        transform: "translateY(-50%)",
      };
    case "center":
      return {
        ...baseStyles,
        top: "50%",
        left: "50%",
        transform: "translate(-50%, -50%)",
      };
    case "bottom-left":
      return { ...baseStyles, bottom: "0.5rem", left: "0.5rem" };
    case "bottom-right":
      return { ...baseStyles, bottom: "0.5rem", right: "0.5rem" };
    case "bottom-full":
      return { ...baseStyles, bottom: "0.5rem", left: "0.5rem", right: "0.5rem" };
    default:
      return baseStyles;
  }
}
