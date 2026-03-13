import { loadBigQueryConfig, tableRef, viewRef } from "./bigquery-config";

describe("loadBigQueryConfig", () => {
  const originalEnv = process.env;

  beforeEach(() => {
    jest.resetModules();
    process.env = { ...originalEnv };
  });

  afterAll(() => {
    process.env = originalEnv;
  });

  it("should throw error when GCP_TEAM_PROJECT_ID is missing", () => {
    delete process.env.GCP_TEAM_PROJECT_ID;

    expect(() => loadBigQueryConfig()).toThrow("GCP_TEAM_PROJECT_ID environment variable is required");
  });

  it("should load config with default values when only project ID is set", () => {
    process.env.GCP_TEAM_PROJECT_ID = "test-project";

    const config = loadBigQueryConfig();

    expect(config.projectId).toBe("test-project");
    expect(config.metricsDataset).toBe("copilot_metrics");
    expect(config.metricsTable).toBe("usage_metrics");
    expect(config.adoptionDataset).toBe("copilot_adoption");
  });

  it("should use custom values when environment variables are set", () => {
    process.env.GCP_TEAM_PROJECT_ID = "custom-project";
    process.env.COPILOT_METRICS_DATASET = "custom_metrics";
    process.env.COPILOT_METRICS_TABLE = "custom_table";
    process.env.COPILOT_ADOPTION_DATASET = "custom_adoption";

    const config = loadBigQueryConfig();

    expect(config.projectId).toBe("custom-project");
    expect(config.metricsDataset).toBe("custom_metrics");
    expect(config.metricsTable).toBe("custom_table");
    expect(config.adoptionDataset).toBe("custom_adoption");
  });
});

describe("tableRef", () => {
  it("should create a properly formatted table reference", () => {
    const result = tableRef("my-project", "my_dataset", "my_table");

    expect(result).toBe("`my-project.my_dataset.my_table`");
  });
});

describe("viewRef", () => {
  it("should create a properly formatted view reference", () => {
    const result = viewRef("my-project", "my_dataset", "my_view");

    expect(result).toBe("`my-project.my_dataset.my_view`");
  });
});
