import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  {
    settings: {
      react: {
        version: "19",
      },
    },
    rules: {
      // Disable false positive for already-escaped quotes in JSX
      "react/no-unescaped-entities": "off",
    },
  },
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "dist/**",
    "storybook-static/**",
    "next-env.d.ts",
    // Generated local video artifacts:
    "video-packages/**",
  ]),
]);

export default eslintConfig;
