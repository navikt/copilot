/**
 * Extra trace-header propagation origins for @nais/apm browser tracing.
 *
 * @nais/apm enforces a non-overridable propagation floor (the app's own origin
 * plus `https://*.nav.no`). These origins are APPENDED to that floor — they can
 * never replace or empty it. We add `*.nav.cloud.nais.io` so browser spans keep
 * joining backend traces on nais ingresses, matching the coverage of the old
 * hand-rolled Faro setup.
 *
 * The patterns are anchored at the start of the URL and terminated at a host
 * boundary (path, port, or end of string) so lookalike hosts such as
 * `https://x.nav.cloud.nais.io.evil.com` do NOT match and trace headers never
 * leak to arbitrary origins (CodeQL alert #31).
 */
export const propagateExtraOrigins: RegExp[] = [/^https:\/\/([a-z0-9-]+\.)*nav\.cloud\.nais\.io(\/|:|$)/];
