import { render, screen } from "@testing-library/react";
import ReviewOutcomes from "./review-outcomes";

describe("ReviewOutcomes", () => {
  it("does not calculate a rate from partially reported comment days", () => {
    render(
      <ReviewOutcomes
        repositories={[
          {
            review_outcome_days: 2,
            review_comment_days: 1,
            review_comments_evaluated: 10,
            review_comments_addressed: 2,
          },
        ]}
      />
    );
    expect(screen.getByText(/Andel fulgt opp: Ikke tilgjengelig/)).toBeInTheDocument();
    expect(screen.getByText(/Summer dekker bare rapporterte felt/)).toBeInTheDocument();
  });
  it("does not present absent historical fields as zero", () => {
    render(<ReviewOutcomes repositories={[{}]} />);
    expect(screen.getByText(/Ingen rapporterte utfall/)).toBeInTheDocument();
    expect(screen.queryByText(/0 %/)).not.toBeInTheDocument();
  });

  it("weights the rate by evaluated comments, not repository percentages", () => {
    render(
      <ReviewOutcomes
        repositories={[
          {
            review_outcome_days: 1,
            review_comment_days: 1,
            review_comments_evaluated: 1,
            review_comments_addressed: 1,
          },
          {
            review_outcome_days: 1,
            review_comment_days: 1,
            review_comments_evaluated: 9,
            review_comments_addressed: 1,
          },
          {},
        ]}
      />
    );
    expect(screen.getByText(/Andel fulgt opp: 20 %/)).toBeInTheDocument();
    expect(screen.getByText(/Gjelder 2 av 3 synlige repoer som oppfyller aktivitetskravet/)).toBeInTheDocument();
  });

  it("keeps zero evaluations unavailable while counting bot-inclusive merges", () => {
    render(
      <ReviewOutcomes
        repositories={[
          {
            review_outcome_days: 1,
            review_comment_days: 1,
            review_comments_evaluated: 0,
            review_comments_addressed: 0,
            review_merged_no_human_change_request: 1,
            review_outcome_first_day: "2026-11-04",
            review_outcome_last_day: "2026-11-05",
          },
        ]}
      />
    );
    expect(screen.getByText(/Andel fulgt opp: Ikke tilgjengelig/)).toHaveTextContent("0 av 0");
    expect(screen.getByText(/PR-er flettet uten senere/)).toHaveTextContent("fra en person: 1");
    expect(screen.getByText(/Rapporterte utfall fra 2026-11-04/)).toBeInTheDocument();
  });

  it("preserves unknown dimension labels and sums their counts", () => {
    const row = {
      review_outcome_days: 1,
      review_breakdowns: [{ dimension: "comment_type", label: "test_coverage", evaluated: 2, addressed: 1 }],
    };
    render(<ReviewOutcomes repositories={[row, row]} />);
    expect(screen.getByText(/Kommentartype: test_coverage/)).toHaveTextContent("2 av 4 kommentarer");
  });
});
