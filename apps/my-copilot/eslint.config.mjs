// For more info, see https://github.com/storybookjs/eslint-plugin-storybook#configuration-flat-config-format
import storybook from "eslint-plugin-storybook";

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
      // No server actions: an action id dies with each deploy, so a page loaded
      // before one cannot send (#1804). Use a route handler. See README.
      "no-restricted-syntax": [
        "error",
        {
          selector: "ExpressionStatement[directive='use server']",
          message: "Ingen server actions: bruk en route handler (#1804, se README).",
        },
      ],
    },
  }, // Override default ignores of eslint-config-next.
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
    // Playwright: the two builds from hack/e2e-build.sh, and the test output.
    ".e2e/**",
    "test-results/**",
    "playwright-report/**",
  ]),
  ...storybook.configs["flat/recommended"],
]);

export default eslintConfig;
