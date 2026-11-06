export interface UsageQualityDay {
  day: string;
  scope: "enterprise" | "organization";
  scope_id: string;
  missing_client_telemetry_user_count?: number | null;
  below_usage_metrics_minimum_user_count?: number | null;
}
